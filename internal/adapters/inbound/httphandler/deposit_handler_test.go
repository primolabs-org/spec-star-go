package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/primolabs-org/spec-star-go/internal/application"
)

type mockDepositExecutor struct {
	resp       *application.DepositResponse
	statusCode int
	err        error
	captured   application.DepositRequest
}

func (m *mockDepositExecutor) Execute(_ context.Context, req application.DepositRequest) (*application.DepositResponse, int, error) {
	m.captured = req
	return m.resp, m.statusCode, m.err
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
	handler := NewDepositHandler(mock)

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
	handler := NewDepositHandler(&mockDepositExecutor{})

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
	handler := NewDepositHandler(&mockDepositExecutor{})

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
	handler := NewDepositHandler(mock)

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
	handler := NewDepositHandler(mock)

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
	handler := NewDepositHandler(mock)

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
	handler := NewDepositHandler(mock)

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
	handler := NewDepositHandler(mock)

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
			handler := NewDepositHandler(tt.mock)
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
	handler := NewDepositHandler(mock)

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
	handler := NewDepositHandler(mock)

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
