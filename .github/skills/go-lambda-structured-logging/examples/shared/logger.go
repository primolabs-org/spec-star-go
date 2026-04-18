package shared

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

type LoggerFactory struct {
	base      *slog.Logger
	service   string
	coldStart bool
}

func NewLoggerFactory(service string, level slog.Leveler) *LoggerFactory {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	base := slog.New(handler).With(
		slog.String("service", service),
	)

	return &LoggerFactory{
		base:      base,
		service:   service,
		coldStart: true,
	}
}

func (f *LoggerFactory) FromContext(ctx context.Context, trigger string, operation string) *slog.Logger {
	logger := f.base.With(
		slog.String("trigger", trigger),
		slog.String("operation", operation),
	)

	if lc, ok := lambdacontext.FromContext(ctx); ok {
		logger = logger.With(slog.String("request_id", lc.AwsRequestID))
	}

	if f.coldStart {
		logger = logger.With(slog.Bool("cold_start", true))
		f.coldStart = false
	}

	return logger
}
