package platform

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitTelemetry initializes OpenTelemetry tracing and metrics providers configured
// via standard OTel environment variables and registers them globally.
// It returns a shutdown function that flushes and closes both providers.
func InitTelemetry(ctx context.Context) (shutdown func(context.Context) error, err error) {
	return initTelemetry(ctx, telemetryDeps{
		buildResource:     buildResource,
		newTraceExporter:  newTraceExporter,
		newMetricExporter: newMetricExporter,
	})
}

type telemetryDeps struct {
	buildResource     func(ctx context.Context) (*resource.Resource, error)
	newTraceExporter  func(ctx context.Context) (sdktrace.SpanExporter, error)
	newMetricExporter func(ctx context.Context) (sdkmetric.Exporter, error)
}

func initTelemetry(ctx context.Context, deps telemetryDeps) (shutdown func(context.Context) error, err error) {
	res, err := deps.buildResource(ctx)
	if err != nil {
		return nil, fmt.Errorf("building otel resource: %w", err)
	}

	traceExp, err := deps.newTraceExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)

	metricExp, err := deps.newMetricExporter(ctx)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("creating metric exporter: %w", err),
			tp.Shutdown(ctx),
		)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	shutdown = func(ctx context.Context) error {
		return errors.Join(
			tp.Shutdown(ctx),
			mp.Shutdown(ctx),
		)
	}

	return shutdown, nil
}

func buildResource(ctx context.Context) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
}

func newTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	return otlptracegrpc.New(ctx)
}

func newMetricExporter(ctx context.Context) (sdkmetric.Exporter, error) {
	return otlpmetricgrpc.New(ctx)
}
