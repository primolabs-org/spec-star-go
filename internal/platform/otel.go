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

// Function variables for testability. Production code uses the defaults;
// tests may override them to inject failures for error-path coverage.
var (
	newResource       = func(ctx context.Context) (*resource.Resource, error) { return buildResource(ctx) }
	newTraceExporter  = func(ctx context.Context) (sdktrace.SpanExporter, error) { return otlptracegrpc.New(ctx) }
	newMetricExporter = func(ctx context.Context) (sdkmetric.Exporter, error) { return otlpmetricgrpc.New(ctx) }
)

// buildResource creates a resource from standard OTel environment variables.
func buildResource(ctx context.Context) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
}

// InitTelemetry initialises OpenTelemetry tracing and metrics providers.
//
// Resource attributes (service.name, service.version, deployment.environment)
// are read from standard OTel environment variables (OTEL_SERVICE_NAME,
// OTEL_RESOURCE_ATTRIBUTES). Exporter endpoints are configured via
// OTEL_EXPORTER_OTLP_ENDPOINT.
//
// The returned shutdown function flushes and shuts down both providers and
// must be called before the process exits.
func InitTelemetry(ctx context.Context) (shutdown func(context.Context) error, err error) {
	res, err := newResource(ctx)
	if err != nil {
		return nil, fmt.Errorf("building otel resource: %w", err)
	}

	traceExporter, err := newTraceExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	metricExporter, err := newMetricExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	shutdown = func(ctx context.Context) error {
		return errors.Join(
			tracerProvider.Shutdown(ctx),
			meterProvider.Shutdown(ctx),
		)
	}

	return shutdown, nil
}
