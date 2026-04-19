package main

import (
	"context"
	"log"

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

	service := application.NewDepositService(clients, assets, positions, processedCommands, unitOfWork)
	handler := httphandler.NewDepositHandler(service)

	lambda.Start(handler.Handle)
}
