package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/aws/aws-lambda-go/events"

	"example/internal/shared"
)

type UseCase interface {
	Execute(ctx context.Context, id string) (string, error)
}

type Handler struct {
	uc UseCase
}

func NewHandler(uc UseCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	id := req.PathParameters["id"]
	value, err := h.uc.Execute(ctx, id)
	if err != nil {
		return mapError(err), nil
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Body:       value,
	}, nil
}

func mapError(err error) events.APIGatewayV2HTTPResponse {
	switch {
	case errors.Is(err, shared.ErrInvalidInput):
		return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusBadRequest, Body: `{"message":"invalid request"}`}
	case errors.Is(err, shared.ErrNotFound):
		return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusNotFound, Body: `{"message":"not found"}`}
	case errors.Is(err, shared.ErrConflict):
		return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusConflict, Body: `{"message":"conflict"}`}
	default:
		return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusInternalServerError, Body: `{"message":"internal server error"}`}
	}
}
