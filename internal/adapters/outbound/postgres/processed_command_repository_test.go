//go:build integration

package postgres

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
)

func TestProcessedCommandRepository_CreateAndFindByTypeAndOrderID(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	repo := NewProcessedCommandRepository(pool)

	orderID := "ORDER-" + uuid.NewString()[:8]
	snapshot := []byte(`{"status":"ok","amount":"100.50"}`)
	cmd, err := domain.NewProcessedCommand("DEPOSIT", orderID, uuid.New(), snapshot)
	if err != nil {
		t.Fatalf("creating processed command: %v", err)
	}
	if err := repo.Create(ctx, cmd); err != nil {
		t.Fatalf("inserting processed command: %v", err)
	}

	got, err := repo.FindByTypeAndOrderID(ctx, "DEPOSIT", orderID)
	if err != nil {
		t.Fatalf("finding processed command: %v", err)
	}

	if got.CommandID() != cmd.CommandID() {
		t.Errorf("command_id: got %s, want %s", got.CommandID(), cmd.CommandID())
	}
	if got.CommandType() != cmd.CommandType() {
		t.Errorf("command_type: got %q, want %q", got.CommandType(), cmd.CommandType())
	}
	if got.OrderID() != cmd.OrderID() {
		t.Errorf("order_id: got %q, want %q", got.OrderID(), cmd.OrderID())
	}
	if got.ClientID() != cmd.ClientID() {
		t.Errorf("client_id: got %s, want %s", got.ClientID(), cmd.ClientID())
	}
	if string(got.ResponseSnapshot()) != string(cmd.ResponseSnapshot()) {
		t.Errorf("response_snapshot: got %s, want %s", got.ResponseSnapshot(), cmd.ResponseSnapshot())
	}
	if !timesEqualMicro(got.CreatedAt(), cmd.CreatedAt()) {
		t.Errorf("created_at: got %v, want %v", got.CreatedAt(), cmd.CreatedAt())
	}
}

func TestProcessedCommandRepository_FindByTypeAndOrderID_NotFound(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	repo := NewProcessedCommandRepository(pool)

	_, err := repo.FindByTypeAndOrderID(ctx, "NONEXISTENT", "NONEXISTENT-"+uuid.NewString()[:8])
	if err == nil {
		t.Fatal("expected error for non-existent command")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestProcessedCommandRepository_CreateDuplicate(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	repo := NewProcessedCommandRepository(pool)

	orderID := "ORDER-DUP-" + uuid.NewString()[:8]
	snapshot := []byte(`{"status":"ok"}`)
	cmd1, err := domain.NewProcessedCommand("DEPOSIT", orderID, uuid.New(), snapshot)
	if err != nil {
		t.Fatalf("creating first command: %v", err)
	}
	cmd2, err := domain.NewProcessedCommand("DEPOSIT", orderID, uuid.New(), snapshot)
	if err != nil {
		t.Fatalf("creating second command: %v", err)
	}

	if err := repo.Create(ctx, cmd1); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	err = repo.Create(ctx, cmd2)
	if err == nil {
		t.Fatal("expected error on duplicate (command_type, order_id)")
	}
	if !errors.Is(err, domain.ErrDuplicate) {
		t.Errorf("expected ErrDuplicate, got: %v", err)
	}
}
