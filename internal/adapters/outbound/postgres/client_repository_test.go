//go:build integration

package postgres

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
)

func TestClientRepository_CreateAndFindByID(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	repo := NewClientRepository(pool)

	client, err := domain.NewClient("EXT-ROUNDTRIP")
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	if err := repo.Create(ctx, client); err != nil {
		t.Fatalf("inserting client: %v", err)
	}

	got, err := repo.FindByID(ctx, client.ClientID())
	if err != nil {
		t.Fatalf("finding client: %v", err)
	}

	if got.ClientID() != client.ClientID() {
		t.Errorf("client_id: got %s, want %s", got.ClientID(), client.ClientID())
	}
	if got.ExternalID() != client.ExternalID() {
		t.Errorf("external_id: got %q, want %q", got.ExternalID(), client.ExternalID())
	}
	if !timesEqualMicro(got.CreatedAt(), client.CreatedAt()) {
		t.Errorf("created_at: got %v, want %v", got.CreatedAt(), client.CreatedAt())
	}
}

func TestClientRepository_FindByID_NotFound(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	repo := NewClientRepository(pool)

	_, err := repo.FindByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent client")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestClientRepository_CreateDuplicate(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	repo := NewClientRepository(pool)

	client, err := domain.NewClient("EXT-DUP")
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	if err := repo.Create(ctx, client); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := repo.Create(ctx, client); err == nil {
		t.Fatal("expected error on duplicate client_id insert")
	}
}
