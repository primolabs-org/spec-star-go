package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// DBTX abstracts the query surface shared by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

const postgresDBSystem = "postgresql"

func executorFromContext(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func stringFromNullable(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func startDBSpan(ctx context.Context, spanName, operation string) (context.Context, trace.Span) {
	return otel.Tracer("postgres").Start(
		ctx,
		spanName,
		trace.WithAttributes(
			attribute.String("db.system", postgresDBSystem),
			attribute.String("db.operation.name", operation),
		),
	)
}
