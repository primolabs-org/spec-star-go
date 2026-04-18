package createorder

import (
    "context"

    "example/internal/domain/order"
    "example/internal/ports/outbound"
)

type Command struct {
    OrderID    string
    CustomerID string
}

type Result struct {
    OrderID string
}

type Handler struct {
    orders outbound.OrderRepository
}

func NewHandler(orders outbound.OrderRepository) Handler {
    return Handler{orders: orders}
}

func (h Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
    entity, err := order.New(order.ID(cmd.OrderID), cmd.CustomerID)
    if err != nil {
        return Result{}, err
    }

    if err := h.orders.Save(ctx, entity); err != nil {
        return Result{}, err
    }

    return Result{OrderID: string(entity.ID())}, nil
}
