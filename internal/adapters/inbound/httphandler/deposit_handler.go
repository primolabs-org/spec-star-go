package httphandler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/primolabs-org/spec-star-go/internal/application"
	"github.com/primolabs-org/spec-star-go/internal/platform"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Package-level OTel tracer and metric instruments shared across handlers.
var (
	tracer             = otel.Tracer("httphandler")
	meter              = otel.Meter("httphandler")
	commandCount, _    = meter.Int64Counter("wallet.command.count",
		metric.WithDescription("Total commands processed"),
	)
	commandDuration, _ = meter.Float64Histogram("wallet.command.duration",
		metric.WithDescription("End-to-end command duration in milliseconds"),
		metric.WithUnit("ms"),
	)
)

// recordCommandMetrics records wallet.command.count and wallet.command.duration.
func recordCommandMetrics(ctx context.Context, command, outcome string, duration time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("command", command),
		attribute.String("outcome", outcome),
	)
	commandCount.Add(ctx, 1, attrs)
	commandDuration.Record(ctx, float64(duration.Milliseconds()), attrs)
}

type depositExecutor interface {
	Execute(ctx context.Context, req application.DepositRequest) (*application.DepositResponse, int, error)
}

// DepositHandler maps API Gateway HTTP API v2 events to DepositService.
type DepositHandler struct {
	service depositExecutor
	loggers loggerFactory
}

// NewDepositHandler constructs a DepositHandler.
func NewDepositHandler(service depositExecutor, loggers loggerFactory) *DepositHandler {
	return &DepositHandler{service: service, loggers: loggers}
}

var jsonMarshal = json.Marshal

type loggerFactory interface {
	FromContext(ctx context.Context, trigger, operation string) *slog.Logger
}

func logTerminalError(logger *slog.Logger, status int, err error) {
	attrs := []any{
		"status", status,
		"error", err.Error(),
		"outcome", "failed",
	}
	if status >= http.StatusInternalServerError {
		logger.Error("request failed", attrs...)
		return
	}
	logger.Warn("request failed", attrs...)
}

// Handle processes an API Gateway HTTP API v2 request.
func (h *DepositHandler) Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	ctx, span := tracer.Start(ctx, "POST /deposits",
		trace.WithAttributes(
			attribute.String("http.method", "POST"),
			attribute.String("http.route", "/deposits"),
			attribute.String("wallet.command", "deposit"),
		),
	)
	defer span.End()
	start := time.Now()

	if req.RequestContext.HTTP.Method != http.MethodPost {
		return errorResponse(http.StatusMethodNotAllowed, "method not allowed")
	}

	var depositReq application.DepositRequest
	if err := json.Unmarshal([]byte(req.Body), &depositReq); err != nil {
		return errorResponse(http.StatusUnprocessableEntity, "invalid request body")
	}

	logger := h.loggers.FromContext(ctx, "http", "deposit")
	ctx = platform.WithLogger(ctx, logger)

	resp, statusCode, err := h.service.Execute(ctx, depositReq)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		span.SetAttributes(attribute.String("wallet.outcome", "failed"))
		recordCommandMetrics(ctx, "deposit", "failed", time.Since(start))
		logTerminalError(logger, statusCode, err)
		return errorResponse(statusCode, err.Error())
	}

	outcome := "success"
	if statusCode == http.StatusOK {
		outcome = "replayed"
	}
	span.SetStatus(codes.Ok, "")
	span.SetAttributes(attribute.String("wallet.outcome", outcome))
	recordCommandMetrics(ctx, "deposit", outcome, time.Since(start))

	return jsonResponse(statusCode, resp)
}

func jsonResponse(statusCode int, body any) (events.APIGatewayV2HTTPResponse, error) {
	data, err := jsonMarshal(body)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{}, fmt.Errorf("marshal response body: %w", err)
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(data),
	}, nil
}

func errorResponse(statusCode int, message string) (events.APIGatewayV2HTTPResponse, error) {
	return jsonResponse(statusCode, map[string]string{"error": message})
}
