package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
)

func TestNewClient_ValidCreation(t *testing.T) {
	before := time.Now()
	c, err := domain.NewClient("B3-12345")
	after := time.Now()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ClientID() == uuid.Nil {
		t.Fatal("expected non-nil client_id")
	}
	if c.ExternalID() != "B3-12345" {
		t.Fatalf("expected external_id %q, got %q", "B3-12345", c.ExternalID())
	}
	if c.CreatedAt().Before(before) || c.CreatedAt().After(after) {
		t.Fatalf("expected created_at between %v and %v, got %v", before, after, c.CreatedAt())
	}
}

func TestNewClient_MissingExternalID(t *testing.T) {
	_, err := domain.NewClient("")
	if err == nil {
		t.Fatal("expected error for empty external_id")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewClient_UniqueIDs(t *testing.T) {
	c1, err := domain.NewClient("ext-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c2, err := domain.NewClient("ext-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c1.ClientID() == c2.ClientID() {
		t.Fatal("expected distinct client_id values for different clients")
	}
}

func TestReconstructClient(t *testing.T) {
	id := uuid.New()
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	c := domain.ReconstructClient(id, "ext-42", ts)

	if c.ClientID() != id {
		t.Fatalf("expected client_id %v, got %v", id, c.ClientID())
	}
	if c.ExternalID() != "ext-42" {
		t.Fatalf("expected external_id %q, got %q", "ext-42", c.ExternalID())
	}
	if !c.CreatedAt().Equal(ts) {
		t.Fatalf("expected created_at %v, got %v", ts, c.CreatedAt())
	}
}
