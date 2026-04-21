package platform

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type noopSpanExporter struct{}

func (noopSpanExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error { return nil }
func (noopSpanExporter) Shutdown(context.Context) error                             { return nil }

type noopMetricExporter struct{}

func (noopMetricExporter) Temporality(sdkmetric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}
func (noopMetricExporter) Aggregation(sdkmetric.InstrumentKind) sdkmetric.Aggregation { return nil }
func (noopMetricExporter) Export(context.Context, *metricdata.ResourceMetrics) error  { return nil }
func (noopMetricExporter) ForceFlush(context.Context) error                           { return nil }
func (noopMetricExporter) Shutdown(context.Context) error                             { return nil }

func noopDeps() telemetryDeps {
	return telemetryDeps{
		buildResource: func(ctx context.Context) (*resource.Resource, error) {
			return resource.Default(), nil
		},
		newTraceExporter: func(context.Context) (sdktrace.SpanExporter, error) {
			return noopSpanExporter{}, nil
		},
		newMetricExporter: func(context.Context) (sdkmetric.Exporter, error) {
			return noopMetricExporter{}, nil
		},
	}
}

func TestInitTelemetry_ReturnsShutdownAndNoError(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "test-service")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.version=1.0.0,deployment.environment=test")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_TIMEOUT", "1")

	shutdown, err := InitTelemetry(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Logf("shutdown error (expected without collector): %v", err)
	}
}

func TestInitTelemetry_ShutdownReturnsNoError(t *testing.T) {
	shutdown, err := initTelemetry(context.Background(), noopDeps())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}

func TestInitTelemetry_SetsGlobalTracerProvider(t *testing.T) {
	shutdown, err := initTelemetry(context.Background(), noopDeps())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown(context.Background())

	tp := otel.GetTracerProvider()
	if _, ok := tp.(*sdktrace.TracerProvider); !ok {
		t.Fatalf("expected *sdktrace.TracerProvider, got %T", tp)
	}
}

func TestInitTelemetry_SetsGlobalMeterProvider(t *testing.T) {
	shutdown, err := initTelemetry(context.Background(), noopDeps())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown(context.Background())

	mp := otel.GetMeterProvider()
	if _, ok := mp.(*sdkmetric.MeterProvider); !ok {
		t.Fatalf("expected *sdkmetric.MeterProvider, got %T", mp)
	}
}

func TestBuildResource_IncludesServiceName(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "test-service")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")

	res, err := buildResource(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, attr := range res.Attributes() {
		if string(attr.Key) == "service.name" && attr.Value.AsString() == "test-service" {
			return
		}
	}
	t.Fatalf("resource does not contain service.name=test-service, got: %v", res.Attributes())
}

func TestInitTelemetry_ResourceBuildError(t *testing.T) {
	wantErr := errors.New("resource failure")
	deps := telemetryDeps{
		buildResource: func(context.Context) (*resource.Resource, error) {
			return nil, wantErr
		},
		newTraceExporter: func(context.Context) (sdktrace.SpanExporter, error) {
			t.Fatal("newTraceExporter should not be called")
			return nil, nil
		},
		newMetricExporter: func(context.Context) (sdkmetric.Exporter, error) {
			t.Fatal("newMetricExporter should not be called")
			return nil, nil
		},
	}

	_, err := initTelemetry(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped resource error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "building otel resource") {
		t.Fatalf("expected error message to contain 'building otel resource', got: %v", err)
	}
}

func TestInitTelemetry_TraceExporterError(t *testing.T) {
	wantErr := errors.New("trace exporter failure")
	deps := telemetryDeps{
		buildResource: func(ctx context.Context) (*resource.Resource, error) {
			return resource.Default(), nil
		},
		newTraceExporter: func(context.Context) (sdktrace.SpanExporter, error) {
			return nil, wantErr
		},
		newMetricExporter: func(context.Context) (sdkmetric.Exporter, error) {
			t.Fatal("newMetricExporter should not be called")
			return nil, nil
		},
	}

	_, err := initTelemetry(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped trace exporter error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "creating trace exporter") {
		t.Fatalf("expected error message to contain 'creating trace exporter', got: %v", err)
	}
}

func TestInitTelemetry_MetricExporterError(t *testing.T) {
	wantErr := errors.New("metric exporter failure")
	deps := telemetryDeps{
		buildResource: func(ctx context.Context) (*resource.Resource, error) {
			return resource.Default(), nil
		},
		newTraceExporter: func(context.Context) (sdktrace.SpanExporter, error) {
			return noopSpanExporter{}, nil
		},
		newMetricExporter: func(context.Context) (sdkmetric.Exporter, error) {
			return nil, wantErr
		},
	}

	_, err := initTelemetry(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped metric exporter error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "creating metric exporter") {
		t.Fatalf("expected error message to contain 'creating metric exporter', got: %v", err)
	}
}
