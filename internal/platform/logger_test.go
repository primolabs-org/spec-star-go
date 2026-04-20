package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/aws/aws-lambda-go/lambdacontext"
	oteltrace "go.opentelemetry.io/otel/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func newTestFactory(buf *bytes.Buffer, service string) *LoggerFactory {
	return newLoggerFactory(service, slog.LevelDebug, buf)
}

func parseJSONLine(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid JSON log line: %v\nraw: %s", err, data)
	}
	return m
}

func emitAndParse(t *testing.T, logger *slog.Logger, buf *bytes.Buffer) map[string]any {
	t.Helper()
	buf.Reset()
	logger.Info("test")
	return parseJSONLine(t, buf.Bytes())
}

func TestNewLoggerFactory_FromContext_EmitsJSONWithServiceField(t *testing.T) {
	var buf bytes.Buffer
	f := newTestFactory(&buf, "test-service")

	logger := f.FromContext(context.Background(), "http", "deposit")
	entry := emitAndParse(t, logger, &buf)

	if got, ok := entry["service"]; !ok || got != "test-service" {
		t.Errorf("expected service=test-service, got %v", got)
	}
}

func TestFromContext_WithLambdaContext_IncludesAllFields(t *testing.T) {
	var buf bytes.Buffer
	f := newTestFactory(&buf, "test-service")

	lc := &lambdacontext.LambdaContext{AwsRequestID: "req-123"}
	ctx := lambdacontext.NewContext(context.Background(), lc)

	logger := f.FromContext(ctx, "http", "deposit")
	entry := emitAndParse(t, logger, &buf)

	expected := map[string]any{
		"request_id": "req-123",
		"trigger":    "http",
		"operation":  "deposit",
	}
	for key, want := range expected {
		got, ok := entry[key]
		if !ok {
			t.Errorf("missing field %q", key)
			continue
		}
		if got != want {
			t.Errorf("field %q: got %v, want %v", key, got, want)
		}
	}

	coldStart, ok := entry["cold_start"]
	if !ok {
		t.Fatal("missing field cold_start")
	}
	if coldStart != true {
		t.Errorf("expected cold_start=true, got %v", coldStart)
	}
}

func TestFromContext_WithoutLambdaContext_OmitsRequestID(t *testing.T) {
	var buf bytes.Buffer
	f := newTestFactory(&buf, "test-service")
	// consume cold start
	f.FromContext(context.Background(), "http", "warmup")

	logger := f.FromContext(context.Background(), "http", "deposit")
	entry := emitAndParse(t, logger, &buf)

	if _, ok := entry["request_id"]; ok {
		t.Error("expected request_id to be absent without Lambda context")
	}

	if got := entry["trigger"]; got != "http" {
		t.Errorf("expected trigger=http, got %v", got)
	}
	if got := entry["operation"]; got != "deposit" {
		t.Errorf("expected operation=deposit, got %v", got)
	}
}

func TestColdStart_FirstCallTrue_SecondCallFalse(t *testing.T) {
	var buf bytes.Buffer
	f := newTestFactory(&buf, "test-service")

	logger1 := f.FromContext(context.Background(), "http", "op1")
	entry1 := emitAndParse(t, logger1, &buf)

	if entry1["cold_start"] != true {
		t.Errorf("first call: expected cold_start=true, got %v", entry1["cold_start"])
	}

	logger2 := f.FromContext(context.Background(), "http", "op2")
	entry2 := emitAndParse(t, logger2, &buf)

	if entry2["cold_start"] != false {
		t.Errorf("second call: expected cold_start=false, got %v", entry2["cold_start"])
	}
}

func TestWithLogger_LoggerFromContext_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler).With("custom", "value")

	ctx := WithLogger(context.Background(), logger)
	got := LoggerFromContext(ctx)

	entry := emitAndParse(t, got, &buf)

	if entry["custom"] != "value" {
		t.Errorf("round-trip failed: expected custom=value, got %v", entry["custom"])
	}
}

func TestLoggerFromContext_ReturnsDefaultWhenAbsent(t *testing.T) {
	got := LoggerFromContext(context.Background())
	if got != slog.Default() {
		t.Error("expected slog.Default() when no logger in context")
	}
}

func TestNewLoggerFactory_SetsSlogDefault(t *testing.T) {
	var buf bytes.Buffer
	_ = newTestFactory(&buf, "default-test")

	buf.Reset()
	slog.Info("test from default")
	entry := parseJSONLine(t, buf.Bytes())

	if got, ok := entry["service"]; !ok || got != "default-test" {
		t.Errorf("slog.Default() should have service=default-test, got %v", got)
	}
}

func TestNewLoggerFactory_PublicConstructor(t *testing.T) {
	f := NewLoggerFactory("public-test", slog.LevelInfo)

	logger := f.FromContext(context.Background(), "http", "op")
	if logger == nil {
		t.Fatal("expected non-nil logger from public constructor")
	}
}

func emitAndParseCtx(t *testing.T, logger *slog.Logger, ctx context.Context, buf *bytes.Buffer) map[string]any {
	t.Helper()
	buf.Reset()
	logger.InfoContext(ctx, "test")
	return parseJSONLine(t, buf.Bytes())
}

func TestTraceCorrelation_RecordingSpan_IncludesTraceFields(t *testing.T) {
	var buf bytes.Buffer
	f := newTestFactory(&buf, "test-service")

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-op")
	defer span.End()

	logger := f.FromContext(ctx, "http", "deposit")
	entry := emitAndParseCtx(t, logger, ctx, &buf)

	sc := span.SpanContext()
	wantTraceID := sc.TraceID().String()
	wantSpanID := sc.SpanID().String()

	if got := entry["trace_id"]; got != wantTraceID {
		t.Errorf("trace_id: got %v, want %s", got, wantTraceID)
	}
	if got := entry["span_id"]; got != wantSpanID {
		t.Errorf("span_id: got %v, want %s", got, wantSpanID)
	}
}

func TestTraceCorrelation_NoSpan_OmitsTraceFields(t *testing.T) {
	var buf bytes.Buffer
	f := newTestFactory(&buf, "test-service")

	logger := f.FromContext(context.Background(), "http", "deposit")
	entry := emitAndParseCtx(t, logger, context.Background(), &buf)

	if _, ok := entry["trace_id"]; ok {
		t.Error("expected trace_id to be absent without a span")
	}
	if _, ok := entry["span_id"]; ok {
		t.Error("expected span_id to be absent without a span")
	}
}

func TestTraceCorrelation_NoopSpan_OmitsTraceFields(t *testing.T) {
	var buf bytes.Buffer
	f := newTestFactory(&buf, "test-service")

	noopTP := oteltrace.NewNoopTracerProvider()
	ctx, span := noopTP.Tracer("test").Start(context.Background(), "noop-op")
	defer span.End()

	logger := f.FromContext(ctx, "http", "deposit")
	entry := emitAndParseCtx(t, logger, ctx, &buf)

	if _, ok := entry["trace_id"]; ok {
		t.Error("expected trace_id to be absent with noop span")
	}
	if _, ok := entry["span_id"]; ok {
		t.Error("expected span_id to be absent with noop span")
	}
}

func TestTraceContextHandler_WithGroup_PreservesTraceCorrelation(t *testing.T) {
	var buf bytes.Buffer
	f := newTestFactory(&buf, "test-service")

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	ctx, span := tp.Tracer("test").Start(context.Background(), "group-op")
	defer span.End()

	logger := f.base.WithGroup("mygroup")
	buf.Reset()
	logger.InfoContext(ctx, "test", "key", "val")
	entry := parseJSONLine(t, buf.Bytes())

	sc := span.SpanContext()

	group, ok := entry["mygroup"].(map[string]any)
	if !ok {
		t.Fatal("expected mygroup group in output")
	}
	if got := group["trace_id"]; got != sc.TraceID().String() {
		t.Errorf("trace_id: got %v, want %s", got, sc.TraceID().String())
	}
	if got := group["span_id"]; got != sc.SpanID().String() {
		t.Errorf("span_id: got %v, want %s", got, sc.SpanID().String())
	}
	if group["key"] != "val" {
		t.Errorf("expected mygroup.key=val, got %v", group["key"])
	}
}

func TestSetDefault_InheritsTraceCorrelation(t *testing.T) {
	var buf bytes.Buffer
	_ = newTestFactory(&buf, "default-trace-test")

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	ctx, span := tp.Tracer("test").Start(context.Background(), "default-op")
	defer span.End()

	buf.Reset()
	slog.InfoContext(ctx, "test from default")
	entry := parseJSONLine(t, buf.Bytes())

	sc := span.SpanContext()
	if got := entry["trace_id"]; got != sc.TraceID().String() {
		t.Errorf("trace_id: got %v, want %s", got, sc.TraceID().String())
	}
	if got := entry["span_id"]; got != sc.SpanID().String() {
		t.Errorf("span_id: got %v, want %s", got, sc.SpanID().String())
	}
}
