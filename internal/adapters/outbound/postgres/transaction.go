package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/primolabs-org/spec-star-go/internal/ports"
)

var _ ports.UnitOfWork = (*TransactionRunner)(nil)

type TransactionRunner struct {
	pool *pgxpool.Pool
}

func NewTransactionRunner(pool *pgxpool.Pool) *TransactionRunner {
	return &TransactionRunner{pool: pool}
}

func (r *TransactionRunner) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}
