package httphandler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/primolabs-org/spec-star-go/internal/application"
	"github.com/primolabs-org/spec-star-go/internal/platform"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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

// --- OTel test infrastructure (shared across all test files in package) ---

var (
	testSpanExporter *tracetest.InMemoryExporter
	testMetricReader *sdkmetric.ManualReader
	initTestOTel     sync.Once
)

func setupTestOTel(t *testing.T) (*tracetest.InMemoryExporter, *sdkmetric.ManualReader) {
	t.Helper()
	initTestOTel.Do(func() {
		testSpanExporter = tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(testSpanExporter)),
		)
		otel.SetTracerProvider(tp)

		testMetricReader = sdkmetric.NewManualReader()
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(testMetricReader),
		)
		otel.SetMeterProvider(mp)
	})
	testSpanExporter.Reset()
	return testSpanExporter, testMetricReader
}

func assertSpanAttribute(t *testing.T, span tracetest.SpanStub, key, expected string) {
	t.Helper()
	for _, attr := range span.Attributes {
		if string(attr.Key) == key {
			if attr.Value.AsString() != expected {
				t.Errorf("expected attribute %s=%q, got %q", key, expected, attr.Value.AsString())
			}
			return
		}
	}
	t.Errorf("attribute %s not found in span", key)
}

func assertCommandCountMetricExists(t *testing.T, reader *sdkmetric.ManualReader) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "wallet.command.count" {
				return
			}
		}
	}
	t.Error("wallet.command.count metric not found")
}

// --- Deposit handler span and metric tests ---

func TestHandle_SuccessfulDeposit_CreatesSpanWithStatusOKAndAttributes(t *testing.T) {
	exp, _ := setupTestOTel(t)
	mock := &mockDepositExecutor{
		resp:       &application.DepositResponse{PositionID: "p1"},
		statusCode: http.StatusCreated,
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	_, err := handler.Handle(context.Background(), postRequest(validBody()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name != "POST /deposits" {
		t.Errorf("expected span name %q, got %q", "POST /deposits", span.Name)
	}
	if span.Status.Code != codes.Ok {
		t.Errorf("expected span status OK, got %v", span.Status.Code)
	}
	assertSpanAttribute(t, span, "http.method", "POST")
	assertSpanAttribute(t, span, "http.route", "/deposits")
	assertSpanAttribute(t, span, "wallet.command", "deposit")
	assertSpanAttribute(t, span, "wallet.outcome", "success")
}

func TestHandle_FailedDeposit_ValidationError_CreatesSpanWithStatusError(t *testing.T) {
	exp, _ := setupTestOTel(t)
	mock := &mockDepositExecutor{
		statusCode: http.StatusUnprocessableEntity,
		err:        errors.New("amount must be positive"),
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	_, err := handler.Handle(context.Background(), postRequest(validBody()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status.Code != codes.Error {
		t.Errorf("expected span status Error, got %v", span.Status.Code)
	}
	assertSpanAttribute(t, span, "wallet.outcome", "failed")
}

func TestHandle_FailedDeposit_ServiceError_CreatesSpanWithStatusError(t *testing.T) {
	exp, _ := setupTestOTel(t)
	mock := &mockDepositExecutor{
		statusCode: http.StatusInternalServerError,
		err:        errors.New("db connection lost"),
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	_, err := handler.Handle(context.Background(), postRequest(validBody()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status.Code != codes.Error {
		t.Errorf("expected span status Error, got %v", span.Status.Code)
	}
	if span.Status.Description != "db connection lost" {
		t.Errorf("expected span status description %q, got %q", "db connection lost", span.Status.Description)
	}
	assertSpanAttribute(t, span, "wallet.outcome", "failed")
}

func TestHandle_IdempotentReplay_SpanOutcomeReplayed(t *testing.T) {
	exp, _ := setupTestOTel(t)
	mock := &mockDepositExecutor{
		resp:       &application.DepositResponse{PositionID: "p2"},
		statusCode: http.StatusOK,
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	_, err := handler.Handle(context.Background(), postRequest(validBody()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	assertSpanAttribute(t, spans[0], "wallet.outcome", "replayed")
}

func TestHandle_SuccessfulDeposit_RecordsCommandCount(t *testing.T) {
	_, reader := setupTestOTel(t)
	mock := &mockDepositExecutor{
		resp:       &application.DepositResponse{PositionID: "p1"},
		statusCode: http.StatusCreated,
	}
	handler := NewDepositHandler(mock, newMockLoggerFactory())

	_, err := handler.Handle(context.Background(), postRequest(validBody()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertCommandCountMetricExists(t, reader)
}
