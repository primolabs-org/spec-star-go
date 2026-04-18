package ports

import (
	"context"

	"github.com/primolabs-org/spec-star-go/internal/domain"
)

// ProcessedCommandRepository defines persistence operations for ProcessedCommand entities.
type ProcessedCommandRepository interface {
	FindByTypeAndOrderID(ctx context.Context, commandType, orderID string) (*domain.ProcessedCommand, error)
	Create(ctx context.Context, cmd *domain.ProcessedCommand) error
}
