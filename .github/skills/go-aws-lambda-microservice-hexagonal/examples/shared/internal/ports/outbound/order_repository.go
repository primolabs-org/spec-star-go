package outbound

import (
    "context"

    "example/internal/domain/order"
)

type OrderRepository interface {
    Save(ctx context.Context, entity order.Order) error
}
