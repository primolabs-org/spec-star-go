package httpapi

import (
    "context"
    "fmt"
    "time"

    "github.com/aws/aws-lambda-go/events"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/metric"
)

type CreateOrderHandler interface {
    Handle(ctx context.Context, input CreateOrderInput) error
}

type CreateOrderInput struct {
    OrderID string
}

type Handler struct {
    useCase      CreateOrderHandler
    tracer       = otel.Tracer("orders.httpapi")
    meter        = otel.Meter("orders.httpapi")
    latency      metric.Float64Histogram
    failureCount metric.Int64Counter
}

func New(useCase CreateOrderHandler) (*Handler, error) {
    meter := otel.Meter("orders.httpapi")
    latency, err := meter.Float64Histogram("orders.http.server.duration_ms")
    if err != nil {
        return nil, fmt.Errorf("create duration instrument: %w", err)
    }
    failures, err := meter.Int64Counter("orders.http.server.failures")
    if err != nil {
        return nil, fmt.Errorf("create failure instrument: %w", err)
    }

    return &Handler{useCase: useCase, latency: latency, failureCount: failures}, nil
}

func (h *Handler) Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
    start := time.Now()
    ctx, span := otel.Tracer("orders.httpapi").Start(ctx, "http create-order")
    defer span.End()

    attrs := []attribute.KeyValue{
        attribute.String("trigger", "http"),
        attribute.String("http.method", req.RequestContext.HTTP.Method),
        attribute.String("http.route", req.RawPath),
    }
    span.SetAttributes(attrs...)

    err := h.useCase.Handle(ctx, CreateOrderInput{OrderID: req.PathParameters["id"]})
    duration := float64(time.Since(start).Milliseconds())
    h.latency.Record(ctx, duration, metric.WithAttributes(attrs...))

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "create order failed")
        h.failureCount.Add(ctx, 1, metric.WithAttributes(attrs...))
        return events.APIGatewayV2HTTPResponse{StatusCode: 500, Body: `{"message":"internal error"}`}, nil
    }

    span.SetStatus(codes.Ok, "ok")
    return events.APIGatewayV2HTTPResponse{StatusCode: 201}, nil
}
