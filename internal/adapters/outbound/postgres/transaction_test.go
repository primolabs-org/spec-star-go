//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/primolabs-org/spec-star-go/internal/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTransactionRunner_Commit(t *testing.T) {
	pool := testPool(t)
	runner := NewTransactionRunner(pool)
	exporter := setupPostgresTestTracer(t)
	clientRepo := NewClientRepository(pool)

	client, err := domain.NewClient("EXT-TX-COMMIT")
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM clients WHERE client_id = $1", client.ClientID())
	})

	err = runner.Do(context.Background(), func(ctx context.Context) error {
		return clientRepo.Create(ctx, client)
	})
	if err != nil {
		t.Fatalf("Do should succeed: %v", err)
	}

	got, err := clientRepo.FindByID(context.Background(), client.ClientID())
	if err != nil {
		t.Fatalf("client should be visible after commit: %v", err)
	}
	if got.ClientID() != client.ClientID() {
		t.Errorf("client_id: got %s, want %s", got.ClientID(), client.ClientID())
	}

	requireDBSpanSuccess(t, exporter, "db.transaction", "TRANSACTION")
}

func TestTransactionRunner_Rollback(t *testing.T) {
	pool := testPool(t)
	runner := NewTransactionRunner(pool)
	exporter := setupPostgresTestTracer(t)
	clientRepo := NewClientRepository(pool)

	client, err := domain.NewClient("EXT-TX-ROLLBACK")
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	intentional := fmt.Errorf("intentional failure")
	err = runner.Do(context.Background(), func(ctx context.Context) error {
		if err := clientRepo.Create(ctx, client); err != nil {
			return err
		}
		return intentional
	})
	if !errors.Is(err, intentional) {
		t.Fatalf("Do should return fn error: got %v", err)
	}

	_, err = clientRepo.FindByID(context.Background(), client.ClientID())
	if err == nil {
		t.Fatal("client should not be visible after rollback")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}

	requireDBSpanError(t, exporter, "db.transaction", "TRANSACTION")
}

func TestTransactionRunner_BeginError(t *testing.T) {
	pool := testPool(t)
	runner := NewTransactionRunner(pool)
	exporter := setupPostgresTestTracer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runner.Do(ctx, func(_ context.Context) error { return nil })
	if err == nil {
		t.Fatal("expected begin transaction error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	requireDBSpanError(t, exporter, "db.transaction", "TRANSACTION")
}

func TestTransactionRunner_NestedRepositoryCalls(t *testing.T) {
	pool := testPool(t)
	runner := NewTransactionRunner(pool)
	clientRepo := NewClientRepository(pool)

	client, err := domain.NewClient("EXT-TX-NESTED")
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM clients WHERE client_id = $1", client.ClientID())
	})

	err = runner.Do(context.Background(), func(ctx context.Context) error {
		if err := clientRepo.Create(ctx, client); err != nil {
			return err
		}
		got, err := clientRepo.FindByID(ctx, client.ClientID())
		if err != nil {
			return fmt.Errorf("finding client within tx: %w", err)
		}
		if got.ClientID() != client.ClientID() {
			return fmt.Errorf("client_id mismatch within tx")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do with nested calls should succeed: %v", err)
	}
}

func TestTransactionRunner_RollbackFailure_JoinsBothErrors(t *testing.T) {
	pool := testPool(t)
	runner := NewTransactionRunner(pool)

	fnError := fmt.Errorf("fn failed")
	ctx, cancel := context.WithCancel(context.Background())

	err := runner.Do(ctx, func(_ context.Context) error {
		cancel()
		return fnError
	})
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !errors.Is(err, fnError) {
		t.Errorf("expected fn error to be preserved, got: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled to be surfaced, got: %v", err)
	}
}

func setupPostgresTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)

	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	})

	return exporter
}

func requireDBSpanSuccess(t *testing.T, exporter *tracetest.InMemoryExporter, spanName, operation string) {
	t.Helper()

	span := requireSpanByName(t, exporter, spanName)
	requireSpanAttribute(t, span, "db.system", postgresDBSystem)
	requireSpanAttribute(t, span, "db.operation.name", operation)

	if span.Status.Code != codes.Ok {
		t.Fatalf("expected span status OK for %q, got %v", spanName, span.Status.Code)
	}
}

func requireDBSpanError(t *testing.T, exporter *tracetest.InMemoryExporter, spanName, operation string) {
	t.Helper()

	span := requireSpanByName(t, exporter, spanName)
	requireSpanAttribute(t, span, "db.system", postgresDBSystem)
	requireSpanAttribute(t, span, "db.operation.name", operation)

	if span.Status.Code != codes.Error {
		t.Fatalf("expected span status Error for %q, got %v", spanName, span.Status.Code)
	}
	if span.Status.Description == "" {
		t.Fatalf("expected non-empty error status description for %q", spanName)
	}
	if !spanHasExceptionEvent(span) {
		t.Fatalf("expected exception event for %q", spanName)
	}
}

func requireSpanByName(t *testing.T, exporter *tracetest.InMemoryExporter, spanName string) tracetest.SpanStub {
	t.Helper()

	spans := exporter.GetSpans()
	for i := len(spans) - 1; i >= 0; i-- {
		if spans[i].Name == spanName {
			return spans[i]
		}
	}

	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name)
	}
	t.Fatalf("expected span %q, got spans %v", spanName, names)
	return tracetest.SpanStub{}
}

func requireSpanAttribute(t *testing.T, span tracetest.SpanStub, key, expected string) {
	t.Helper()

	for _, attr := range span.Attributes {
		if string(attr.Key) != key {
			continue
		}
		if attr.Value.AsString() != expected {
			t.Fatalf("span attribute %q: expected %q, got %q", key, expected, attr.Value.AsString())
		}
		return
	}

	t.Fatalf("span attribute %q not found", key)
}

func spanHasExceptionEvent(span tracetest.SpanStub) bool {
	for _, event := range span.Events {
		if event.Name == "exception" {
			return true
		}
	}
	return false
}
