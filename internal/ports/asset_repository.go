package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
)

// AssetRepository defines persistence operations for Asset entities.
type AssetRepository interface {
	FindByID(ctx context.Context, assetID uuid.UUID) (*domain.Asset, error)
	FindByInstrumentID(ctx context.Context, instrumentID string) (*domain.Asset, error)
	Create(ctx context.Context, asset *domain.Asset) error
}
