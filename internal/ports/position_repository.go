package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
)

// PositionRepository defines persistence operations for Position entities.
type PositionRepository interface {
	FindByID(ctx context.Context, positionID uuid.UUID) (*domain.Position, error)
	FindByClientAndAsset(ctx context.Context, clientID, assetID uuid.UUID) ([]*domain.Position, error)
	FindByClientAndInstrument(ctx context.Context, clientID uuid.UUID, instrumentID string) ([]*domain.Position, error)
	Create(ctx context.Context, position *domain.Position) error
	Update(ctx context.Context, position *domain.Position) error
}
