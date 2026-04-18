package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
)

// ClientRepository defines persistence operations for Client entities.
type ClientRepository interface {
	FindByID(ctx context.Context, clientID uuid.UUID) (*domain.Client, error)
	Create(ctx context.Context, client *domain.Client) error
}
