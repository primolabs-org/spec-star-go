//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/primolabs-org/spec-star-go/internal/domain"
)

func TestTransactionRunner_Commit(t *testing.T) {
	pool := testPool(t)
	runner := NewTransactionRunner(pool)
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
}

func TestTransactionRunner_Rollback(t *testing.T) {
	pool := testPool(t)
	runner := NewTransactionRunner(pool)
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
