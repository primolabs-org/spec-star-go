package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/primolabs-org/spec-star-go/internal/adapters/inbound/httphandler"
	"github.com/primolabs-org/spec-star-go/internal/adapters/outbound/postgres"
	"github.com/primolabs-org/spec-star-go/internal/application"
	"github.com/primolabs-org/spec-star-go/internal/platform"
)

func main() {
	cfg, err := platform.LoadDatabaseConfig()
	if err != nil {
		log.Fatalf("loading database config: %v", err)
	}

	pool, err := platform.NewPool(context.Background(), cfg)
	if err != nil {
		log.Fatalf("creating database pool: %v", err)
	}

	clients := postgres.NewClientRepository(pool)
	assets := postgres.NewAssetRepository(pool)
	positions := postgres.NewPositionRepository(pool)
	processedCommands := postgres.NewProcessedCommandRepository(pool)
	unitOfWork := postgres.NewTransactionRunner(pool)

	depositService := application.NewDepositService(clients, assets, positions, processedCommands, unitOfWork)
	depositHandler := httphandler.NewDepositHandler(depositService)

	withdrawService := application.NewWithdrawService(clients, positions, processedCommands, unitOfWork)
	withdrawHandler := httphandler.NewWithdrawHandler(withdrawService)

	route := func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		switch req.RequestContext.HTTP.Path {
		case "/deposits":
			return depositHandler.Handle(ctx, req)
		case "/withdrawals":
			return withdrawHandler.Handle(ctx, req)
		default:
			return events.APIGatewayV2HTTPResponse{
				StatusCode: 404,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"error": "not found"}`,
			}, nil
		}
	}

	lambda.Start(route)
}
