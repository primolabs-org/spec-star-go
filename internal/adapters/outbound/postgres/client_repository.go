package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/primolabs-org/spec-star-go/internal/platform"
	"github.com/primolabs-org/spec-star-go/internal/ports"
	"go.opentelemetry.io/otel/codes"
)

var _ ports.ClientRepository = (*ClientRepository)(nil)

type ClientRepository struct {
	pool *pgxpool.Pool
}

func NewClientRepository(pool *pgxpool.Pool) *ClientRepository {
	return &ClientRepository{pool: pool}
}

func (r *ClientRepository) FindByID(ctx context.Context, clientID uuid.UUID) (*domain.Client, error) {
	ctx, span := startDBSpan(ctx, "db.client.find_by_id", "SELECT")
	defer span.End()

	db := executorFromContext(ctx, r.pool)

	var (
		id         uuid.UUID
		externalID string
		createdAt  time.Time
	)
	err := db.QueryRow(ctx,
		`SELECT client_id, external_id, created_at FROM clients WHERE client_id = $1`,
		clientID,
	).Scan(&id, &externalID, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf("client %s: %w", clientID, domain.ErrNotFound)
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
			return nil, err
		}
		platform.LoggerFromContext(ctx).Error("FindByID: query failed", "client_id", clientID.String(), "error", err.Error())
		err = fmt.Errorf("querying client %s: %w", clientID, err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return domain.ReconstructClient(id, externalID, createdAt), nil
}

func (r *ClientRepository) Create(ctx context.Context, client *domain.Client) error {
	db := executorFromContext(ctx, r.pool)

	_, err := db.Exec(ctx,
		`INSERT INTO clients (client_id, external_id, created_at) VALUES ($1, $2, $3)`,
		client.ClientID(), client.ExternalID(), client.CreatedAt(),
	)
	if err != nil {
		platform.LoggerFromContext(ctx).Error("Create: exec failed", "client_id", client.ClientID().String(), "error", err.Error())
		return fmt.Errorf("inserting client %s: %w", client.ClientID(), err)
	}
	return nil
}
