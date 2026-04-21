package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/primolabs-org/spec-star-go/internal/platform"
	"github.com/primolabs-org/spec-star-go/internal/ports"
	"go.opentelemetry.io/otel/codes"
)

var _ ports.UnitOfWork = (*TransactionRunner)(nil)

type beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type TransactionRunner struct {
	pool beginner
}

func NewTransactionRunner(pool *pgxpool.Pool) *TransactionRunner {
	return &TransactionRunner{pool: pool}
}

func (r *TransactionRunner) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	ctx, span := startDBSpan(ctx, "db.transaction", "TRANSACTION")
	defer span.End()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		platform.LoggerFromContext(ctx).ErrorContext(ctx, "Do: begin transaction failed", "error", err.Error())
		err = fmt.Errorf("beginning transaction: %w", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return err
	}

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			err = errors.Join(err, rbErr)
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
			return err
		}
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		platform.LoggerFromContext(ctx).ErrorContext(ctx, "Do: commit transaction failed", "error", err.Error())
		err = fmt.Errorf("committing transaction: %w", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return err
	}
	span.SetStatus(codes.Ok, "")
	return nil
}
