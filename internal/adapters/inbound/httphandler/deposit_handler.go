package httphandler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/primolabs-org/spec-star-go/internal/application"
	"github.com/primolabs-org/spec-star-go/internal/platform"
)

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
		logTerminalError(logger, statusCode, err)
		return errorResponse(statusCode, err.Error())
	}

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
