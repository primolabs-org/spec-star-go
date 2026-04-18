package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
)

func TestNewProcessedCommand_ValidCreation(t *testing.T) {
	clientID := uuid.New()
	snapshot := []byte(`{"status":"ok"}`)

	before := time.Now()
	pc, err := domain.NewProcessedCommand("DEPOSIT", "order-123", clientID, snapshot)
	after := time.Now()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pc.CommandID() == uuid.Nil {
		t.Fatal("expected non-nil command_id")
	}
	if pc.CommandType() != "DEPOSIT" {
		t.Fatalf("expected command_type %q, got %q", "DEPOSIT", pc.CommandType())
	}
	if pc.OrderID() != "order-123" {
		t.Fatalf("expected order_id %q, got %q", "order-123", pc.OrderID())
	}
	if pc.ClientID() != clientID {
		t.Fatalf("expected client_id %v, got %v", clientID, pc.ClientID())
	}
	if string(pc.ResponseSnapshot()) != `{"status":"ok"}` {
		t.Fatalf("expected response_snapshot %q, got %q", `{"status":"ok"}`, string(pc.ResponseSnapshot()))
	}
	if pc.CreatedAt().Before(before) || pc.CreatedAt().After(after) {
		t.Fatal("expected created_at within bounds")
	}
}

func TestNewProcessedCommand_MissingCommandType(t *testing.T) {
	_, err := domain.NewProcessedCommand("", "order-123", uuid.New(), []byte(`data`))
	if err == nil {
		t.Fatal("expected error for empty command_type")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewProcessedCommand_MissingOrderID(t *testing.T) {
	_, err := domain.NewProcessedCommand("DEPOSIT", "", uuid.New(), []byte(`data`))
	if err == nil {
		t.Fatal("expected error for empty order_id")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewProcessedCommand_MissingClientID(t *testing.T) {
	_, err := domain.NewProcessedCommand("DEPOSIT", "order-123", uuid.Nil, []byte(`data`))
	if err == nil {
		t.Fatal("expected error for nil client_id")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewProcessedCommand_EmptyResponseSnapshot(t *testing.T) {
	_, err := domain.NewProcessedCommand("DEPOSIT", "order-123", uuid.New(), []byte{})
	if err == nil {
		t.Fatal("expected error for empty response_snapshot")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewProcessedCommand_NilResponseSnapshot(t *testing.T) {
	_, err := domain.NewProcessedCommand("DEPOSIT", "order-123", uuid.New(), nil)
	if err == nil {
		t.Fatal("expected error for nil response_snapshot")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestReconstructProcessedCommand(t *testing.T) {
	id := uuid.New()
	clientID := uuid.New()
	snapshot := []byte(`{"result":"value"}`)
	created := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	pc := domain.ReconstructProcessedCommand(id, "WITHDRAW", "order-456", clientID, snapshot, created)

	if pc.CommandID() != id {
		t.Fatalf("expected command_id %v, got %v", id, pc.CommandID())
	}
	if pc.CommandType() != "WITHDRAW" {
		t.Fatalf("expected command_type %q, got %q", "WITHDRAW", pc.CommandType())
	}
	if pc.OrderID() != "order-456" {
		t.Fatalf("expected order_id %q, got %q", "order-456", pc.OrderID())
	}
	if pc.ClientID() != clientID {
		t.Fatalf("expected client_id %v, got %v", clientID, pc.ClientID())
	}
	if string(pc.ResponseSnapshot()) != `{"result":"value"}` {
		t.Fatalf("expected response_snapshot %q, got %q", `{"result":"value"}`, string(pc.ResponseSnapshot()))
	}
	if !pc.CreatedAt().Equal(created) {
		t.Fatalf("expected created_at %v, got %v", created, pc.CreatedAt())
	}
}
