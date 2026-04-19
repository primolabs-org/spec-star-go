package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/primolabs-org/spec-star-go/internal/application"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/primolabs-org/spec-star-go/internal/platform"
)

type withdrawExecutor interface {
	Execute(ctx context.Context, req application.WithdrawRequest) (*application.WithdrawResponse, int, error)
}

// WithdrawHandler maps API Gateway HTTP API v2 events to WithdrawService.
type WithdrawHandler struct {
	service withdrawExecutor
	loggers loggerFactory
}

// NewWithdrawHandler constructs a WithdrawHandler.
func NewWithdrawHandler(service withdrawExecutor, loggers loggerFactory) *WithdrawHandler {
	return &WithdrawHandler{service: service, loggers: loggers}
}

// Handle processes an API Gateway HTTP API v2 request.
func (h *WithdrawHandler) Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if req.RequestContext.HTTP.Method != http.MethodPost {
		return errorResponse(http.StatusMethodNotAllowed, "method not allowed")
	}

	var withdrawReq application.WithdrawRequest
	if err := json.Unmarshal([]byte(req.Body), &withdrawReq); err != nil {
		return errorResponse(http.StatusUnprocessableEntity, "invalid request body")
	}

	logger := h.loggers.FromContext(ctx, "http", "withdraw")
	ctx = platform.WithLogger(ctx, logger)

	resp, statusCode, err := h.service.Execute(ctx, withdrawReq)
	if err != nil {
		logTerminalError(logger, statusCode, err)
		if errors.Is(err, domain.ErrInsufficientPosition) {
			return codedErrorResponse(statusCode, err.Error(), "INSUFFICIENT_POSITION")
		}
		if errors.Is(err, domain.ErrConcurrencyConflict) {
			return codedErrorResponse(statusCode, err.Error(), "CONCURRENCY_CONFLICT")
		}
		return errorResponse(statusCode, err.Error())
	}

	return jsonResponse(statusCode, resp)
}

func codedErrorResponse(statusCode int, message, code string) (events.APIGatewayV2HTTPResponse, error) {
	return jsonResponse(statusCode, map[string]string{"error": message, "code": code})
}
