package sqs

import (
	"context"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
)

type App interface {
	ProcessThing(ctx context.Context, input ProcessThingInput) error
}

type ProcessThingInput struct {
	MessageID string
	Body      string
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

func (h *Handler) Handle(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
	base := h.loggers.FromContext(ctx, "sqs", "process_thing").With(
		slog.Int("batch_size", len(event.Records)),
	)

	var failures []events.SQSBatchItemFailure

	for _, record := range event.Records {
		logger := base.With(
			slog.String("message_id", record.MessageId),
			slog.String("queue_arn", record.EventSourceARN),
		)

		err := h.app.ProcessThing(ctx, ProcessThingInput{
			MessageID: record.MessageId,
			Body:      record.Body,
		})
		if err != nil {
			logger.Error("message processing failed",
				slog.String("outcome", "failed"),
				slog.String("error", err.Error()),
			)
			failures = append(failures, events.SQSBatchItemFailure{
				ItemIdentifier: record.MessageId,
			})
			continue
		}

		// Avoid per-message success logs in high-throughput consumers unless explicitly required.
	}

	if len(failures) > 0 {
		base.Warn("batch completed with failures",
			slog.String("outcome", "partial_failure"),
			slog.Int("failed_count", len(failures)),
		)
	} else {
		base.Info("batch completed",
			slog.String("outcome", "success"),
		)
	}

	return events.SQSEventResponse{BatchItemFailures: failures}, nil
}
