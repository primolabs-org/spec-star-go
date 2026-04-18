package dynamodb

import (
    "context"
    "fmt"

    "example/internal/domain/order"
)

type PutItemAPI interface {
    PutItem(ctx context.Context, item map[string]string) error
}

type OrderRepository struct {
    client PutItemAPI
    table  string
}

func NewOrderRepository(client PutItemAPI, table string) OrderRepository {
    return OrderRepository{client: client, table: table}
}

func (r OrderRepository) Save(ctx context.Context, entity order.Order) error {
    item := map[string]string{
        "pk":          fmt.Sprintf("ORDER#%s", entity.ID()),
        "customer_id": entity.CustomerID(),
        "table":       r.table,
    }

    return r.client.PutItem(ctx, item)
}
