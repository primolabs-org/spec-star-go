package httphandler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/primolabs-org/spec-star-go/internal/application"
)

type depositExecutor interface {
	Execute(ctx context.Context, req application.DepositRequest) (*application.DepositResponse, int, error)
}

// DepositHandler maps API Gateway HTTP API v2 events to DepositService.
type DepositHandler struct {
	service depositExecutor
}

// NewDepositHandler constructs a DepositHandler.
func NewDepositHandler(service depositExecutor) *DepositHandler {
	return &DepositHandler{service: service}
}

var jsonMarshal = json.Marshal

// Handle processes an API Gateway HTTP API v2 request.
func (h *DepositHandler) Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if req.RequestContext.HTTP.Method != http.MethodPost {
		return errorResponse(http.StatusMethodNotAllowed, "method not allowed")
	}

	var depositReq application.DepositRequest
	if err := json.Unmarshal([]byte(req.Body), &depositReq); err != nil {
		return errorResponse(http.StatusUnprocessableEntity, "invalid request body")
	}

	resp, statusCode, err := h.service.Execute(ctx, depositReq)
	if err != nil {
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
