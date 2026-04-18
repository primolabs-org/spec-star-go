package bootstrap

import (
    "context"

    "example/internal/adapters/outbound/dynamodb"
    "example/internal/application/createorder"
    "example/internal/platform/logging"
)

type PutItemAPI interface {
    PutItem(ctx context.Context, item map[string]string) error
}

type App struct {
    CreateOrder createorder.Handler
}

func MustBuild(client PutItemAPI, table string) App {
    _ = logging.NewJSONLogger()

    repo := dynamodb.NewOrderRepository(client, table)

    return App{
        CreateOrder: createorder.NewHandler(repo),
    }
}
