package httphandler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/primolabs-org/spec-star-go/internal/application"
	"github.com/primolabs-org/spec-star-go/internal/platform"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type mockDepositExecutor struct {
	resp        *application.DepositResponse
	statusCode  int
	err         error
	captured    application.DepositRequest
	capturedCtx context.Context
}

func (m *mockDepositExecutor) Execute(ctx context.Context, req application.DepositRequest) (*application.DepositResponse, int, error) {
	m.captured = req
	m.capturedCtx = ctx
	return m.resp, m.statusCode, m.err
}

type mockLoggerFactory struct {
	buf *bytes.Buffer
}

func newMockLoggerFactory() *mockLoggerFactory {
	return &mockLoggerFactory{buf: &bytes.Buffer{}}
}

func (m *mockLoggerFactory) FromContext(_ context.Context, trigger, operation string) *slog.Logger {
	handler := slog.NewJSONHandler(m.buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler).With("trigger", trigger, "operation", operation)
}

func (m *mockLoggerFactory) parseLastEntry(t *testing.T) map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(m.buf.Bytes()), []byte("\n"))
	if len(lines) == 0 || len(lines[0]) == 0 {
		t.Fatal("no log entries found")
	}
	var entry map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &entry); err != nil {
		t.Fatalf("invalid JSON log line: %v\nraw: %s", err, lines[len(lines)-1])
	}
	return entry
}

func validBody() string {
	return `{"client_id":"c1","asset_id":"a1","order_id":"o1","amount":"100","unit_price":"10"}`
}

func postRequest(body string) events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		Body: body,
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: http.MethodPost,
			},
		},
	}
}

func requestWithMethod(method string) events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: method,
			},
		},
	}
}

func TestHandle_ValidPost_DelegatesToExecute(t *testing.T) {
	mock := &mockDepositExecutor{
		resp: &application.DepositResponse{
			PositionID: "p1",
			ClientID:   "c1",
			AssetID:    "a1",
			Amount:     "100",
			UnitPrice:  "10",
			TotalValue: "1000",
		},
		statusCode: http.StatusCreated,
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
	if mock.captured.ClientID != "c1" {
		t.Fatalf("expected client_id c1, got %s", mock.captured.ClientID)
	}
	if mock.captured.AssetID != "a1" {
		t.Fatalf("expected asset_id a1, got %s", mock.captured.AssetID)
	}

	var body application.DepositResponse
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}
	if body.PositionID != "p1" {
		t.Fatalf("expected position_id p1, got %s", body.PositionID)
	}
}

func TestHandle_NonPostMethods_Returns405(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	handler := NewDepositHandler(&mockDepositExecutor{}, newMockLoggerFactory())

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			resp, err := handler.Handle(context.Background(), requestWithMethod(method))

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("expected status 405, got %d", resp.StatusCode)
			}
			assertErrorBody(t, resp.Body, "method not allowed")
		})
	}
}

func TestHandle_MalformedJSON_Returns422(t *testing.T) {
	handler := NewDepositHandler(&mockDepositExecutor{}, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest("{invalid"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", resp.StatusCode)
	}
	assertErrorBody(t, resp.Body, "invalid request body")
}

func TestHandle_ExecuteReturns201_Success(t *testing.T) {
	mock := &mockDepositExecutor{
		resp:       &application.DepositResponse{PositionID: "p1", CreatedAt: "2026-01-01T00:00:00Z"},
		statusCode: http.StatusCreated,
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var body application.DepositResponse
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body.PositionID != "p1" {
		t.Fatalf("expected position_id p1, got %s", body.PositionID)
	}
}

func TestHandle_ExecuteReturns200_IdempotentReplay(t *testing.T) {
	mock := &mockDepositExecutor{
		resp:       &application.DepositResponse{PositionID: "p2"},
		statusCode: http.StatusOK,
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body application.DepositResponse
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body.PositionID != "p2" {
		t.Fatalf("expected position_id p2, got %s", body.PositionID)
	}
}

func TestHandle_ExecuteReturns422_ValidationError(t *testing.T) {
	mock := &mockDepositExecutor{
		statusCode: http.StatusUnprocessableEntity,
		err:        errors.New("amount must be positive"),
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", resp.StatusCode)
	}
	assertErrorBody(t, resp.Body, "amount must be positive")
}

func TestHandle_ExecuteReturns409_ConflictError(t *testing.T) {
	mock := &mockDepositExecutor{
		statusCode: http.StatusConflict,
		err:        errors.New("replay after race: not found"),
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", resp.StatusCode)
	}
	assertErrorBody(t, resp.Body, "replay after race: not found")
}

func TestHandle_ExecuteReturns500_InternalError(t *testing.T) {
	mock := &mockDepositExecutor{
		statusCode: http.StatusInternalServerError,
		err:        errors.New("unit of work: connection refused"),
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", resp.StatusCode)
	}
	assertErrorBody(t, resp.Body, "unit of work: connection refused")
}

func TestHandle_AllResponses_HaveContentTypeJSON(t *testing.T) {
	tests := []struct {
		name string
		req  events.APIGatewayV2HTTPRequest
		mock *mockDepositExecutor
	}{
		{
			name: "success",
			req:  postRequest(validBody()),
			mock: &mockDepositExecutor{
				resp:       &application.DepositResponse{PositionID: "p1"},
				statusCode: http.StatusCreated,
			},
		},
		{
			name: "method not allowed",
			req:  requestWithMethod(http.MethodGet),
			mock: &mockDepositExecutor{},
		},
		{
			name: "malformed body",
			req:  postRequest("{bad"),
			mock: &mockDepositExecutor{},
		},
		{
			name: "execute error",
			req:  postRequest(validBody()),
			mock: &mockDepositExecutor{
				statusCode: http.StatusUnprocessableEntity,
				err:        errors.New("validation error"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewDepositHandler(tt.mock, newMockLoggerFactory())
			resp, err := handler.Handle(context.Background(), tt.req)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			ct := resp.Headers["Content-Type"]
			if ct != "application/json" {
				t.Fatalf("expected Content-Type application/json, got %s", ct)
			}
		})
	}
}

func TestHandle_ReadsMethodFromRequestContextHTTP(t *testing.T) {
	mock := &mockDepositExecutor{}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	req := events.APIGatewayV2HTTPRequest{
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: http.MethodGet,
			},
		},
	}

	resp, err := handler.Handle(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 from RequestContext.HTTP.Method, got %d", resp.StatusCode)
	}
}

func TestHandle_MarshalFailure_ReturnsError(t *testing.T) {
	original := jsonMarshal
	t.Cleanup(func() { jsonMarshal = original })
	jsonMarshal = func(v any) ([]byte, error) {
		return nil, errors.New("marshal boom")
	}

	mock := &mockDepositExecutor{
		resp:       &application.DepositResponse{PositionID: "p1"},
		statusCode: http.StatusCreated,
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	_, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err == nil {
		t.Fatal("expected error from marshal failure, got nil")
	}
	if !errors.Is(err, err) {
		t.Fatalf("unexpected error chain: %v", err)
	}
	expected := "marshal response body: marshal boom"
	if err.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, err.Error())
	}
}

func assertErrorBody(t *testing.T, body string, expectedMsg string) {
	t.Helper()
	var parsed map[string]string
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal error body: %v", err)
	}
	if parsed["error"] != expectedMsg {
		t.Fatalf("expected error %q, got %q", expectedMsg, parsed["error"])
	}
}

func TestHandle_ExecuteReturns500_LogsErrorLevel(t *testing.T) {
	logFactory := newMockLoggerFactory()
	mock := &mockDepositExecutor{
		statusCode: http.StatusInternalServerError,
		err:        errors.New("db connection lost"),
	}
	handler := NewDepositHandler(mock, logFactory)

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", resp.StatusCode)
	}

	entry := logFactory.parseLastEntry(t)
	if entry["level"] != "ERROR" {
		t.Errorf("expected level ERROR, got %v", entry["level"])
	}
	if entry["status"] != float64(http.StatusInternalServerError) {
		t.Errorf("expected status 500, got %v", entry["status"])
	}
	if entry["error"] != "db connection lost" {
		t.Errorf("expected error 'db connection lost', got %v", entry["error"])
	}
	if entry["outcome"] != "failed" {
		t.Errorf("expected outcome=failed, got %v", entry["outcome"])
	}
}

func TestHandle_ExecuteReturns4xx_LogsWarnLevel(t *testing.T) {
	logFactory := newMockLoggerFactory()
	mock := &mockDepositExecutor{
		statusCode: http.StatusUnprocessableEntity,
		err:        errors.New("amount must be positive"),
	}
	handler := NewDepositHandler(mock, logFactory)

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", resp.StatusCode)
	}

	entry := logFactory.parseLastEntry(t)
	if entry["level"] != "WARN" {
		t.Errorf("expected level WARN, got %v", entry["level"])
	}
	if entry["status"] != float64(http.StatusUnprocessableEntity) {
		t.Errorf("expected status 422, got %v", entry["status"])
	}
	if entry["error"] != "amount must be positive" {
		t.Errorf("expected error 'amount must be positive', got %v", entry["error"])
	}
	if entry["outcome"] != "failed" {
		t.Errorf("expected outcome=failed, got %v", entry["outcome"])
	}
}

func TestHandle_ServiceExecute_ReceivesContextWithLogger(t *testing.T) {
	mock := &mockDepositExecutor{
		resp:       &application.DepositResponse{PositionID: "p1"},
		statusCode: http.StatusCreated,
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	_, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logger := platform.LoggerFromContext(mock.capturedCtx)
	if logger == slog.Default() {
		t.Fatal("expected enriched logger in context, got slog.Default()")
	}
}

func setupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prev := tracer
	tracer = tp.Tracer("httphandler")
	t.Cleanup(func() {
		tracer = prev
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	})
	return exporter
}

func assertSpanAttribute(t *testing.T, span tracetest.SpanStub, key, expected string) {
	t.Helper()
	for _, attr := range span.Attributes {
		if string(attr.Key) == key {
			if attr.Value.AsString() != expected {
				t.Errorf("span attribute %q: expected %q, got %q", key, expected, attr.Value.AsString())
			}
			return
		}
	}
	t.Errorf("span attribute %q not found", key)
}

type failingCounterMeter struct {
	metricnoop.Meter
}

func (failingCounterMeter) Int64Counter(string, ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return nil, errors.New("counter creation failed")
}

type failingHistogramMeter struct {
	metricnoop.Meter
}

func (failingHistogramMeter) Float64Histogram(string, ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	return nil, errors.New("histogram creation failed")
}

func TestHandle_SuccessfulDeposit_CreatesSpanWithStatusOK(t *testing.T) {
	exporter := setupTestTracer(t)
	mock := &mockDepositExecutor{
		resp:       &application.DepositResponse{PositionID: "p1"},
		statusCode: http.StatusCreated,
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name != "POST /deposits" {
		t.Fatalf("expected span name 'POST /deposits', got %q", span.Name)
	}
	if span.Status.Code != codes.Ok {
		t.Fatalf("expected span status OK, got %v", span.Status.Code)
	}
	assertSpanAttribute(t, span, "http.method", "POST")
	assertSpanAttribute(t, span, "http.route", "/deposits")
	assertSpanAttribute(t, span, "wallet.command", "deposit")
	assertSpanAttribute(t, span, "wallet.outcome", "success")
}

func TestHandle_FailedDeposit_ValidationError_SetsSpanError(t *testing.T) {
	exporter := setupTestTracer(t)
	mock := &mockDepositExecutor{
		statusCode: http.StatusUnprocessableEntity,
		err:        errors.New("amount must be positive"),
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", resp.StatusCode)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Status.Code != codes.Error {
		t.Fatalf("expected span status Error, got %v", span.Status.Code)
	}
	if span.Status.Description != "amount must be positive" {
		t.Fatalf("expected status description 'amount must be positive', got %q", span.Status.Description)
	}
	assertSpanAttribute(t, span, "wallet.outcome", "failed")
}

func TestHandle_FailedDeposit_ServiceError_SetsSpanError(t *testing.T) {
	exporter := setupTestTracer(t)
	mock := &mockDepositExecutor{
		statusCode: http.StatusInternalServerError,
		err:        errors.New("database connection lost"),
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", resp.StatusCode)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Status.Code != codes.Error {
		t.Fatalf("expected span status Error, got %v", span.Status.Code)
	}
	assertSpanAttribute(t, span, "wallet.outcome", "failed")
}

func TestHandle_IdempotentReplay_SetsOutcomeReplayed(t *testing.T) {
	exporter := setupTestTracer(t)
	mock := &mockDepositExecutor{
		resp:       &application.DepositResponse{PositionID: "p2"},
		statusCode: http.StatusOK,
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	resp, err := handler.Handle(context.Background(), postRequest(validBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Status.Code != codes.Ok {
		t.Fatalf("expected span status OK, got %v", span.Status.Code)
	}
	assertSpanAttribute(t, span, "wallet.outcome", "replayed")
}

func TestHandle_FailedDeposit_SpanRecordsError(t *testing.T) {
	exporter := setupTestTracer(t)
	mock := &mockDepositExecutor{
		statusCode: http.StatusInternalServerError,
		err:        errors.New("something broke"),
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	_, err := handler.Handle(context.Background(), postRequest(validBody()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	found := false
	for _, event := range spans[0].Events {
		if event.Name == "exception" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected error event on span")
	}
}

func TestCreateMetrics_CounterCreationFails_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if !strings.Contains(msg, "wallet.command.count") {
			t.Fatalf("expected panic about wallet.command.count, got %q", msg)
		}
	}()
	createMetrics(failingCounterMeter{})
}

func TestCreateMetrics_HistogramCreationFails_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if !strings.Contains(msg, "wallet.command.duration") {
			t.Fatalf("expected panic about wallet.command.duration, got %q", msg)
		}
	}()
	createMetrics(failingHistogramMeter{})
}
