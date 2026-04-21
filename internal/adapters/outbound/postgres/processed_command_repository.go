package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/primolabs-org/spec-star-go/internal/platform"
	"github.com/primolabs-org/spec-star-go/internal/ports"
	"go.opentelemetry.io/otel/codes"
)

const pgUniqueViolation = "23505"

var _ ports.ProcessedCommandRepository = (*ProcessedCommandRepository)(nil)

type ProcessedCommandRepository struct {
	pool *pgxpool.Pool
}

func NewProcessedCommandRepository(pool *pgxpool.Pool) *ProcessedCommandRepository {
	return &ProcessedCommandRepository{pool: pool}
}

func (r *ProcessedCommandRepository) FindByTypeAndOrderID(ctx context.Context, commandType, orderID string) (*domain.ProcessedCommand, error) {
	ctx, span := startDBSpan(ctx, "db.processed_command.find_by_type_and_order_id", "SELECT")
	defer span.End()

	db := executorFromContext(ctx, r.pool)

	var (
		commandID        uuid.UUID
		ct               string
		oid              string
		clientID         uuid.UUID
		responseSnapshot []byte
		createdAt        time.Time
	)
	err := db.QueryRow(ctx,
		`SELECT command_id, command_type, order_id, client_id, response_snapshot, created_at
		 FROM processed_commands WHERE command_type = $1 AND order_id = $2`,
		commandType, orderID,
	).Scan(&commandID, &ct, &oid, &clientID, &responseSnapshot, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf("processed command (%s, %s): %w", commandType, orderID, domain.ErrNotFound)
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
			return nil, err
		}
		platform.LoggerFromContext(ctx).Error("FindByTypeAndOrderID: query failed", "command_type", commandType, "order_id", orderID, "error", err.Error())
		err = fmt.Errorf("querying processed command (%s, %s): %w", commandType, orderID, err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return domain.ReconstructProcessedCommand(commandID, ct, oid, clientID, responseSnapshot, createdAt), nil
}

func (r *ProcessedCommandRepository) Create(ctx context.Context, cmd *domain.ProcessedCommand) error {
	ctx, span := startDBSpan(ctx, "db.processed_command.create", "INSERT")
	defer span.End()

	db := executorFromContext(ctx, r.pool)

	_, err := db.Exec(ctx,
		`INSERT INTO processed_commands (command_id, command_type, order_id, client_id, response_snapshot, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		cmd.CommandID(), cmd.CommandType(), cmd.OrderID(), cmd.ClientID(), cmd.ResponseSnapshot(), cmd.CreatedAt(),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			err = fmt.Errorf("processed command (%s, %s): %w", cmd.CommandType(), cmd.OrderID(), domain.ErrDuplicate)
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
			return err
		}
		platform.LoggerFromContext(ctx).Error("Create: exec failed", "command_id", cmd.CommandID().String(), "error", err.Error())
		err = fmt.Errorf("inserting processed command %s: %w", cmd.CommandID(), err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return err
	}
	span.SetStatus(codes.Ok, "")
	return nil
}
