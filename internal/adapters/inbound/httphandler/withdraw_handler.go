package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/primolabs-org/spec-star-go/internal/application"
)

type withdrawExecutor interface {
	Execute(ctx context.Context, req application.WithdrawRequest) (*application.WithdrawResponse, int, error)
}

// WithdrawHandler maps API Gateway HTTP API v2 events to WithdrawService.
type WithdrawHandler struct {
	service withdrawExecutor
}

// NewWithdrawHandler constructs a WithdrawHandler.
func NewWithdrawHandler(service withdrawExecutor) *WithdrawHandler {
	return &WithdrawHandler{service: service}
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

	resp, statusCode, err := h.service.Execute(ctx, withdrawReq)
	if err != nil {
		var insufficientErr *application.InsufficientPositionError
		if errors.As(err, &insufficientErr) {
			return codedErrorResponse(statusCode, err.Error(), "INSUFFICIENT_POSITION")
		}

		var concurrencyErr *application.ConcurrencyConflictError
		if errors.As(err, &concurrencyErr) {
			return codedErrorResponse(statusCode, err.Error(), "CONCURRENCY_CONFLICT")
		}

		return errorResponse(statusCode, err.Error())
	}

	return jsonResponse(statusCode, resp)
}

func codedErrorResponse(statusCode int, message, errorCode string) (events.APIGatewayV2HTTPResponse, error) {
	return jsonResponse(statusCode, map[string]string{"error": message, "error_code": errorCode})
}
