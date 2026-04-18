package sqs

import (
	"context"
	"errors"

	"github.com/aws/aws-lambda-go/events"

	"example/internal/shared"
)

type Processor interface {
	Execute(ctx context.Context, id string) (string, error)
}

type Handler struct {
	processor Processor
	fifo      bool
}

func NewHandler(processor Processor, fifo bool) *Handler {
	return &Handler{processor: processor, fifo: fifo}
}

func (h *Handler) Handle(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
	failures := make([]events.SQSBatchItemFailure, 0)
	stop := false

	for _, record := range event.Records {
		if stop {
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
			continue
		}

		_, err := h.processor.Execute(ctx, record.MessageId)
		if err == nil {
			continue
		}

		if errors.Is(err, shared.ErrRetryable) {
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
			if h.fifo {
				stop = true
			}
			continue
		}

		if errors.Is(err, shared.ErrTerminal) {
			// terminal failure path intentionally omitted here; teams may choose DLQ/redrive or compensating flow
			continue
		}

		failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
		if h.fifo {
			stop = true
		}
	}

	return events.SQSEventResponse{BatchItemFailures: failures}, nil
}
