package sqs

import (
    "context"
    "time"

    "github.com/aws/aws-lambda-go/events"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/metric"
)

type ProcessMessage interface {
    Handle(ctx context.Context, body string) error
}

type Handler struct {
    useCase      ProcessMessage
    latency      metric.Float64Histogram
    failureCount metric.Int64Counter
}

func New(useCase ProcessMessage) (*Handler, error) {
    meter := otel.Meter("orders.sqs")
    latency, err := meter.Float64Histogram("orders.sqs.record.duration_ms")
    if err != nil {
        return nil, err
    }
    failures, err := meter.Int64Counter("orders.sqs.record.failures")
    if err != nil {
        return nil, err
    }
    return &Handler{useCase: useCase, latency: latency, failureCount: failures}, nil
}

func (h *Handler) Handle(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
    _, batchSpan := otel.Tracer("orders.sqs").Start(ctx, "sqs process batch")
    defer batchSpan.End()

    failures := make([]events.SQSBatchItemFailure, 0)

    for _, record := range event.Records {
        started := time.Now()
        recordCtx, span := otel.Tracer("orders.sqs").Start(ctx, "sqs process record")
        attrs := []attribute.KeyValue{
            attribute.String("trigger", "sqs"),
            attribute.String("message_id", record.MessageId),
            attribute.String("event_source", record.EventSourceARN),
        }
        span.SetAttributes(attrs...)

        err := h.useCase.Handle(recordCtx, record.Body)
        h.latency.Record(recordCtx, float64(time.Since(started).Milliseconds()), metric.WithAttributes(attrs...))
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, "record failed")
            h.failureCount.Add(recordCtx, 1, metric.WithAttributes(attrs...))
            failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
        } else {
            span.SetStatus(codes.Ok, "ok")
        }
        span.End()
    }

    return events.SQSEventResponse{BatchItemFailures: failures}, nil
}
