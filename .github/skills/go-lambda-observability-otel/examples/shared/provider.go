package observability

import (
    "context"
    "fmt"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Config struct {
    ServiceName string
    Environment string
    Version     string
    OTLPEndpoint string
}

type Providers struct {
    TracerProvider *sdktrace.TracerProvider
    MeterProvider  *sdkmetric.MeterProvider
    Shutdown       func(context.Context) error
}

func BuildProviders(ctx context.Context, cfg Config) (*Providers, error) {
    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName(cfg.ServiceName),
            semconv.DeploymentEnvironmentName(cfg.Environment),
            semconv.ServiceVersion(cfg.Version),
            attribute.String("runtime", "aws-lambda"),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("build otel resource: %w", err)
    }

    traceExporter, err := otlptracehttp.New(ctx,
        otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
        otlptracehttp.WithInsecure(),
    )
    if err != nil {
        return nil, fmt.Errorf("build trace exporter: %w", err)
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(traceExporter),
        sdktrace.WithResource(res),
    )

    mp := sdkmetric.NewMeterProvider(
        sdkmetric.WithResource(res),
    )

    otel.SetTracerProvider(tp)
    otel.SetMeterProvider(mp)

    return &Providers{
        TracerProvider: tp,
        MeterProvider:  mp,
        Shutdown: func(ctx context.Context) error {
            if err := tp.Shutdown(ctx); err != nil {
                return err
            }
            return mp.Shutdown(ctx)
        },
    }, nil
}
