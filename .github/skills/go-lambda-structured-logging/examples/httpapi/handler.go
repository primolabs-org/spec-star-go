package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

type App interface {
	HandleCreateThing(ctx context.Context, input CreateThingInput) (CreateThingResult, error)
}

type CreateThingInput struct {
	Name string
}

type CreateThingResult struct {
	ID string
}

type LoggerFactory interface {
	FromContext(ctx context.Context, trigger string, operation string) *slog.Logger
}

type Handler struct {
	app     App
	loggers LoggerFactory
}

func NewHandler(app App, loggers LoggerFactory) *Handler {
	return &Handler{app: app, loggers: loggers}
}

func (h *Handler) Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	logger := h.loggers.FromContext(ctx, "http", "create_thing").With(
		slog.String("method", req.RequestContext.HTTP.Method),
		slog.String("route", req.RawPath),
	)

	input := CreateThingInput{Name: req.QueryStringParameters["name"]}
	result, err := h.app.HandleCreateThing(ctx, input)
	if err != nil {
		logger.Error("create thing failed",
			slog.String("outcome", "failed"),
			slog.String("error", err.Error()),
		)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"message":"internal error"}`,
		}, nil
	}

	logger.Info("create thing completed",
		slog.String("outcome", "success"),
		slog.String("thing_id", result.ID),
	)

	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusCreated,
		Body:       `{"id":"` + result.ID + `"}`,
	}, nil
}
