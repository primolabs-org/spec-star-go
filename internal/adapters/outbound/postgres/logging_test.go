package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/primolabs-org/spec-star-go/internal/platform"
	"github.com/shopspring/decimal"
)

// mockTx implements pgx.Tx so it passes the type assertion in executorFromContext.
// Embedded pgx.Tx provides nil-receiver stubs for unused methods.
type mockTx struct {
	pgx.Tx
	execFn     func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	commitErr  error
}

func (m *mockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return m.execFn(ctx, sql, arguments...)
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.queryFn(ctx, sql, args...)
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return m.queryRowFn(ctx, sql, args...)
}

func (m *mockTx) Commit(ctx context.Context) error {
	return m.commitErr
}

func (m *mockTx) Rollback(ctx context.Context) error {
	return nil
}

// mockRow implements pgx.Row for QueryRow return values.
type mockRow struct {
	scanErr error
}

func (r *mockRow) Scan(_ ...any) error {
	return r.scanErr
}

// mockBeginner implements the beginner interface for TransactionRunner tests.
type mockBeginner struct {
	beginFn func(ctx context.Context) (pgx.Tx, error)
}

func (m *mockBeginner) Begin(ctx context.Context) (pgx.Tx, error) {
	return m.beginFn(ctx)
}

// setupTestContext creates a context with a captured logger and the given mock injected via txKey.
func setupTestContext(mock pgx.Tx) (context.Context, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := platform.WithLogger(context.Background(), logger)
	if mock != nil {
		ctx = context.WithValue(ctx, txKey{}, mock)
	}
	return ctx, &buf
}

// logEntry represents a parsed JSON log line.
type logEntry struct {
	Level   string `json:"level"`
	Msg     string `json:"msg"`
	Error   string `json:"error"`
	entries map[string]any
}

func parseLogEntries(t *testing.T, buf *bytes.Buffer) []logEntry {
	t.Helper()
	var entries []logEntry
	dec := json.NewDecoder(buf)
	for dec.More() {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			t.Fatalf("failed to decode log entry: %v", err)
		}
		e := logEntry{entries: raw}
		if v, ok := raw["level"].(string); ok {
			e.Level = v
		}
		if v, ok := raw["msg"].(string); ok {
			e.Msg = v
		}
		if v, ok := raw["error"].(string); ok {
			e.Error = v
		}
		entries = append(entries, e)
	}
	return entries
}

func requireSingleErrorLog(t *testing.T, buf *bytes.Buffer, wantMsgSubstring string, wantFields map[string]string) {
	t.Helper()
	entries := parseLogEntries(t, buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d: %s", len(entries), buf.String())
	}
	e := entries[0]
	if e.Level != "ERROR" {
		t.Errorf("expected level ERROR, got %s", e.Level)
	}
	if wantMsgSubstring != "" && !containsSubstring(e.Msg, wantMsgSubstring) {
		t.Errorf("expected msg to contain %q, got %q", wantMsgSubstring, e.Msg)
	}
	for key, wantVal := range wantFields {
		gotVal, ok := e.entries[key]
		if !ok {
			t.Errorf("expected field %q in log entry, not found. Entry: %v", key, e.entries)
			continue
		}
		gotStr, _ := gotVal.(string)
		if gotStr != wantVal {
			t.Errorf("field %q: expected %q, got %q", key, wantVal, gotStr)
		}
	}
}

func requireNoLogEntry(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	if buf.Len() > 0 {
		t.Fatalf("expected no log entries, got: %s", buf.String())
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || bytes.Contains([]byte(s), []byte(substr)))
}

var dbErr = errors.New("connection refused")

// --- ClientRepository ---

func TestLoggingClientRepository(t *testing.T) {
	t.Run("FindByID unexpected error logs ERROR", func(t *testing.T) {
		clientID := uuid.New()
		mock := &mockTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return &mockRow{scanErr: dbErr}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewClientRepository(nil)
		_, err := repo.FindByID(ctx, clientID)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "FindByID: query failed", map[string]string{
			"client_id": clientID.String(),
		})
	})

	t.Run("FindByID ErrNoRows no log", func(t *testing.T) {
		mock := &mockTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return &mockRow{scanErr: pgx.ErrNoRows}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewClientRepository(nil)
		_, err := repo.FindByID(ctx, uuid.New())
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
		requireNoLogEntry(t, buf)
	})

	t.Run("Create exec error logs ERROR", func(t *testing.T) {
		client := domain.ReconstructClient(uuid.New(), "ext-1", time.Now())
		mock := &mockTx{
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, dbErr
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewClientRepository(nil)
		err := repo.Create(ctx, client)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "Create: exec failed", map[string]string{
			"client_id": client.ClientID().String(),
		})
	})
}

// --- AssetRepository ---

func TestLoggingAssetRepository(t *testing.T) {
	t.Run("FindByID unexpected error logs ERROR", func(t *testing.T) {
		assetID := uuid.New()
		mock := &mockTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return &mockRow{scanErr: dbErr}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewAssetRepository(nil)
		_, err := repo.FindByID(ctx, assetID)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "FindByID: query failed", map[string]string{
			"asset_id": assetID.String(),
		})
	})

	t.Run("FindByID ErrNoRows no log", func(t *testing.T) {
		mock := &mockTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return &mockRow{scanErr: pgx.ErrNoRows}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewAssetRepository(nil)
		_, err := repo.FindByID(ctx, uuid.New())
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
		requireNoLogEntry(t, buf)
	})

	t.Run("FindByInstrumentID unexpected error logs ERROR", func(t *testing.T) {
		instID := "INST-123"
		mock := &mockTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return &mockRow{scanErr: dbErr}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewAssetRepository(nil)
		_, err := repo.FindByInstrumentID(ctx, instID)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "FindByInstrumentID: query failed", map[string]string{
			"instrument_id": instID,
		})
	})

	t.Run("FindByInstrumentID ErrNoRows no log", func(t *testing.T) {
		mock := &mockTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return &mockRow{scanErr: pgx.ErrNoRows}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewAssetRepository(nil)
		_, err := repo.FindByInstrumentID(ctx, "INST-999")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
		requireNoLogEntry(t, buf)
	})

	t.Run("Create exec error logs ERROR", func(t *testing.T) {
		asset := domain.ReconstructAsset(
			uuid.New(), "INST-1", domain.ProductType("CDB"),
			"", "EMITTER-1", "", "", "Test Asset",
			time.Now(), time.Now().Add(365*24*time.Hour), time.Now(),
		)
		mock := &mockTx{
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, dbErr
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewAssetRepository(nil)
		err := repo.Create(ctx, asset)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "Create: exec failed", map[string]string{
			"asset_id": asset.AssetID().String(),
		})
	})
}

// --- PositionRepository ---

func TestLoggingPositionRepository(t *testing.T) {
	t.Run("FindByID unexpected error logs ERROR", func(t *testing.T) {
		posID := uuid.New()
		mock := &mockTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return &mockRow{scanErr: dbErr}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewPositionRepository(nil)
		_, err := repo.FindByID(ctx, posID)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "FindByID: query failed", map[string]string{
			"position_id": posID.String(),
		})
	})

	t.Run("FindByID ErrNoRows no log", func(t *testing.T) {
		mock := &mockTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return &mockRow{scanErr: pgx.ErrNoRows}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewPositionRepository(nil)
		_, err := repo.FindByID(ctx, uuid.New())
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
		requireNoLogEntry(t, buf)
	})

	t.Run("FindByClientAndAsset query error logs ERROR", func(t *testing.T) {
		clientID := uuid.New()
		assetID := uuid.New()
		mock := &mockTx{
			queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
				return nil, dbErr
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewPositionRepository(nil)
		_, err := repo.FindByClientAndAsset(ctx, clientID, assetID)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "FindByClientAndAsset: query failed", map[string]string{
			"client_id": clientID.String(),
			"asset_id":  assetID.String(),
		})
	})

	t.Run("FindByClientAndInstrument query error logs ERROR", func(t *testing.T) {
		clientID := uuid.New()
		instID := "INST-456"
		mock := &mockTx{
			queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
				return nil, dbErr
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewPositionRepository(nil)
		_, err := repo.FindByClientAndInstrument(ctx, clientID, instID)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "FindByClientAndInstrument: query failed", map[string]string{
			"client_id":     clientID.String(),
			"instrument_id": instID,
		})
	})

	t.Run("Create exec error logs ERROR", func(t *testing.T) {
		pos := newTestPosition(t)
		mock := &mockTx{
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, dbErr
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewPositionRepository(nil)
		err := repo.Create(ctx, pos)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "Create: exec failed", map[string]string{
			"position_id": pos.PositionID().String(),
		})
	})

	t.Run("Update exec error logs ERROR", func(t *testing.T) {
		pos := newTestPosition(t)
		mock := &mockTx{
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, dbErr
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewPositionRepository(nil)
		err := repo.Update(ctx, pos)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "Update: exec failed", map[string]string{
			"position_id": pos.PositionID().String(),
		})
	})

	t.Run("Update zero rows affected no log", func(t *testing.T) {
		pos := newTestPosition(t)
		mock := &mockTx{
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 0"), nil
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewPositionRepository(nil)
		err := repo.Update(ctx, pos)
		if !errors.Is(err, domain.ErrConcurrencyConflict) {
			t.Fatalf("expected ErrConcurrencyConflict, got %v", err)
		}
		requireNoLogEntry(t, buf)
	})
}

func newTestPosition(t *testing.T) *domain.Position {
	t.Helper()
	now := time.Now()
	amount := decimal.NewFromInt(100)
	unitPrice := decimal.NewFromInt(10)
	pos, err := domain.ReconstructPosition(
		uuid.New(), uuid.New(), uuid.New(),
		amount, unitPrice, amount.Mul(unitPrice),
		decimal.Zero, decimal.Zero,
		now, now, now, 1,
	)
	if err != nil {
		t.Fatalf("constructing test position: %v", err)
	}
	return pos
}

// --- ProcessedCommandRepository ---

func TestLoggingProcessedCommandRepository(t *testing.T) {
	t.Run("FindByTypeAndOrderID unexpected error logs ERROR", func(t *testing.T) {
		mock := &mockTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return &mockRow{scanErr: dbErr}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewProcessedCommandRepository(nil)
		_, err := repo.FindByTypeAndOrderID(ctx, "deposit", "ORD-1")
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "FindByTypeAndOrderID: query failed", map[string]string{
			"command_type": "deposit",
			"order_id":     "ORD-1",
		})
	})

	t.Run("FindByTypeAndOrderID ErrNoRows no log", func(t *testing.T) {
		mock := &mockTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return &mockRow{scanErr: pgx.ErrNoRows}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewProcessedCommandRepository(nil)
		_, err := repo.FindByTypeAndOrderID(ctx, "deposit", "ORD-2")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
		requireNoLogEntry(t, buf)
	})

	t.Run("Create unique violation no log", func(t *testing.T) {
		cmd := domain.ReconstructProcessedCommand(
			uuid.New(), "deposit", "ORD-3", uuid.New(), []byte(`{}`), time.Now(),
		)
		mock := &mockTx{
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, &pgconn.PgError{Code: pgUniqueViolation}
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewProcessedCommandRepository(nil)
		err := repo.Create(ctx, cmd)
		if !errors.Is(err, domain.ErrDuplicate) {
			t.Fatalf("expected ErrDuplicate, got %v", err)
		}
		requireNoLogEntry(t, buf)
	})

	t.Run("Create unexpected error logs ERROR", func(t *testing.T) {
		cmd := domain.ReconstructProcessedCommand(
			uuid.New(), "deposit", "ORD-4", uuid.New(), []byte(`{}`), time.Now(),
		)
		mock := &mockTx{
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, dbErr
			},
		}
		ctx, buf := setupTestContext(mock)

		repo := NewProcessedCommandRepository(nil)
		err := repo.Create(ctx, cmd)
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, buf, "Create: exec failed", map[string]string{
			"command_id": cmd.CommandID().String(),
		})
	})
}

// --- TransactionRunner ---

func TestLoggingTransactionRunner(t *testing.T) {
	t.Run("Do begin failure logs ERROR", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		ctx := platform.WithLogger(context.Background(), logger)

		runner := &TransactionRunner{
			pool: &mockBeginner{
				beginFn: func(_ context.Context) (pgx.Tx, error) {
					return nil, dbErr
				},
			},
		}

		err := runner.Do(ctx, func(_ context.Context) error { return nil })
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, &buf, "Do: begin transaction failed", nil)
	})

	t.Run("Do commit failure logs ERROR", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		ctx := platform.WithLogger(context.Background(), logger)

		runner := &TransactionRunner{
			pool: &mockBeginner{
				beginFn: func(_ context.Context) (pgx.Tx, error) {
					return &mockTx{commitErr: dbErr}, nil
				},
			},
		}

		err := runner.Do(ctx, func(_ context.Context) error { return nil })
		if err == nil {
			t.Fatal("expected error")
		}

		requireSingleErrorLog(t, &buf, "Do: commit transaction failed", nil)
	})
}
