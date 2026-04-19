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
			return nil, fmt.Errorf("processed command (%s, %s): %w", commandType, orderID, domain.ErrNotFound)
		}
		platform.LoggerFromContext(ctx).Error("FindByTypeAndOrderID: query failed", "command_type", commandType, "order_id", orderID, "error", err.Error())
		return nil, fmt.Errorf("querying processed command (%s, %s): %w", commandType, orderID, err)
	}

	return domain.ReconstructProcessedCommand(commandID, ct, oid, clientID, responseSnapshot, createdAt), nil
}

func (r *ProcessedCommandRepository) Create(ctx context.Context, cmd *domain.ProcessedCommand) error {
	db := executorFromContext(ctx, r.pool)

	_, err := db.Exec(ctx,
		`INSERT INTO processed_commands (command_id, command_type, order_id, client_id, response_snapshot, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		cmd.CommandID(), cmd.CommandType(), cmd.OrderID(), cmd.ClientID(), cmd.ResponseSnapshot(), cmd.CreatedAt(),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("processed command (%s, %s): %w", cmd.CommandType(), cmd.OrderID(), domain.ErrDuplicate)
		}
		platform.LoggerFromContext(ctx).Error("Create: exec failed", "command_id", cmd.CommandID().String(), "error", err.Error())
		return fmt.Errorf("inserting processed command %s: %w", cmd.CommandID(), err)
	}
	return nil
}
