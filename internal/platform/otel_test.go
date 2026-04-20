package platform

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	collectormetricv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

// stubTraceService is a minimal OTLP trace service that accepts and discards
// export requests. Used exclusively for testing without a real Collector.
type stubTraceService struct {
	collectortracev1.UnimplementedTraceServiceServer
}

func (s *stubTraceService) Export(_ context.Context, _ *collectortracev1.ExportTraceServiceRequest) (*collectortracev1.ExportTraceServiceResponse, error) {
	return &collectortracev1.ExportTraceServiceResponse{}, nil
}

// stubMetricsService is a minimal OTLP metrics service that accepts and
// discards export requests. Used exclusively for testing without a real
// Collector.
type stubMetricsService struct {
	collectormetricv1.UnimplementedMetricsServiceServer
}

func (s *stubMetricsService) Export(_ context.Context, _ *collectormetricv1.ExportMetricsServiceRequest) (*collectormetricv1.ExportMetricsServiceResponse, error) {
	return &collectormetricv1.ExportMetricsServiceResponse{}, nil
}

// startStubCollector starts a gRPC server on a random port that implements
// the OTLP trace and metrics services. It returns the endpoint address
// (host:port) and stops the server when the test finishes.
func startStubCollector(t *testing.T) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	collectortracev1.RegisterTraceServiceServer(srv, &stubTraceService{})
	collectormetricv1.RegisterMetricsServiceServer(srv, &stubMetricsService{})

	go srv.Serve(lis) //nolint:errcheck
	t.Cleanup(srv.GracefulStop)

	return fmt.Sprintf("http://%s", lis.Addr().String())
}

// setupTelemetryEnv configures OTel environment variables for tests and
// starts a stub collector. It resets global providers on cleanup.
func setupTelemetryEnv(t *testing.T, serviceName string) {
	t.Helper()
	endpoint := startStubCollector(t)
	t.Setenv("OTEL_SERVICE_NAME", serviceName)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Cleanup(func() {
		otel.SetTracerProvider(nil)
		otel.SetMeterProvider(nil)
	})
}

func TestInitTelemetry_ReturnsShutdownAndNoError(t *testing.T) {
	setupTelemetryEnv(t, "test-service")

	shutdown, err := InitTelemetry(context.Background())
	if err != nil {
		t.Fatalf("InitTelemetry returned unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("InitTelemetry returned nil shutdown function")
	}
}

func TestInitTelemetry_ShutdownDoesNotError(t *testing.T) {
	setupTelemetryEnv(t, "test-service")

	shutdown, err := InitTelemetry(context.Background())
	if err != nil {
		t.Fatalf("InitTelemetry returned unexpected error: %v", err)
	}

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned unexpected error: %v", err)
	}
}

func TestInitTelemetry_SetsGlobalTracerProvider(t *testing.T) {
	setupTelemetryEnv(t, "test-service")

	shutdown, err := InitTelemetry(context.Background())
	if err != nil {
		t.Fatalf("InitTelemetry returned unexpected error: %v", err)
	}
	defer shutdown(context.Background()) //nolint:errcheck

	tp := otel.GetTracerProvider()
	if _, ok := tp.(*sdktrace.TracerProvider); !ok {
		t.Fatalf("expected global TracerProvider to be *sdktrace.TracerProvider, got %T", tp)
	}
}

func TestInitTelemetry_SetsGlobalMeterProvider(t *testing.T) {
	setupTelemetryEnv(t, "test-service")

	shutdown, err := InitTelemetry(context.Background())
	if err != nil {
		t.Fatalf("InitTelemetry returned unexpected error: %v", err)
	}
	defer shutdown(context.Background()) //nolint:errcheck

	mp := otel.GetMeterProvider()
	if _, ok := mp.(*sdkmetric.MeterProvider); !ok {
		t.Fatalf("expected global MeterProvider to be *sdkmetric.MeterProvider, got %T", mp)
	}
}

func TestInitTelemetry_ResourceIncludesServiceName(t *testing.T) {
	setupTelemetryEnv(t, "my-wallet-api")

	shutdown, err := InitTelemetry(context.Background())
	if err != nil {
		t.Fatalf("InitTelemetry returned unexpected error: %v", err)
	}
	defer shutdown(context.Background()) //nolint:errcheck

	// buildResource uses the same env-based construction that InitTelemetry
	// uses. We call it directly to inspect the resource attributes because
	// TracerProvider does not expose its resource.
	res, err := buildResource(context.Background())
	if err != nil {
		t.Fatalf("buildResource returned unexpected error: %v", err)
	}

	found := false
	for _, attr := range res.Attributes() {
		if attr.Key == attribute.Key("service.name") && attr.Value.AsString() == "my-wallet-api" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected resource to contain service.name=my-wallet-api, got attributes: %v", res.Attributes())
	}
}

func TestInitTelemetry_ResourceBuildError(t *testing.T) {
	setupTelemetryEnv(t, "test-service")

	origNewResource := newResource
	t.Cleanup(func() { newResource = origNewResource })
	newResource = func(_ context.Context) (*resource.Resource, error) {
		return nil, fmt.Errorf("synthetic resource error")
	}

	shutdown, err := InitTelemetry(context.Background())
	if err == nil {
		t.Fatal("expected error when resource build fails")
	}
	if shutdown != nil {
		t.Fatal("expected nil shutdown when resource build fails")
	}
	if !strings.Contains(err.Error(), "building otel resource") {
		t.Fatalf("expected error to mention 'building otel resource', got: %v", err)
	}
}

func TestInitTelemetry_TraceExporterError(t *testing.T) {
	setupTelemetryEnv(t, "test-service")

	origNewTraceExporter := newTraceExporter
	t.Cleanup(func() { newTraceExporter = origNewTraceExporter })
	newTraceExporter = func(_ context.Context) (sdktrace.SpanExporter, error) {
		return nil, fmt.Errorf("synthetic trace exporter error")
	}

	shutdown, err := InitTelemetry(context.Background())
	if err == nil {
		t.Fatal("expected error when trace exporter creation fails")
	}
	if shutdown != nil {
		t.Fatal("expected nil shutdown when trace exporter creation fails")
	}
	if !strings.Contains(err.Error(), "creating trace exporter") {
		t.Fatalf("expected error to mention 'creating trace exporter', got: %v", err)
	}
}

func TestInitTelemetry_MetricExporterError(t *testing.T) {
	setupTelemetryEnv(t, "test-service")

	origNewMetricExporter := newMetricExporter
	t.Cleanup(func() { newMetricExporter = origNewMetricExporter })
	newMetricExporter = func(_ context.Context) (sdkmetric.Exporter, error) {
		return nil, fmt.Errorf("synthetic metric exporter error")
	}

	shutdown, err := InitTelemetry(context.Background())
	if err == nil {
		t.Fatal("expected error when metric exporter creation fails")
	}
	if shutdown != nil {
		t.Fatal("expected nil shutdown when metric exporter creation fails")
	}
	if !strings.Contains(err.Error(), "creating metric exporter") {
		t.Fatalf("expected error to mention 'creating metric exporter', got: %v", err)
	}
}
