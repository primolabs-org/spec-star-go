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

type mockWithdrawExecutor struct {
	resp       *application.WithdrawResponse
	statusCode int
	err        error
	captured   application.WithdrawRequest
}

func (m *mockWithdrawExecutor) Execute(_ context.Context, req application.WithdrawRequest) (*application.WithdrawResponse, int, error) {
	m.captured = req
	return m.resp, m.statusCode, m.err
}

func withdrawPostRequest(body string) events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		Body: body,
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: http.MethodPost,
			},
		},
	}
}

func withdrawRequestWithMethod(method string) events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: method,
			},
		},
	}
}

func validWithdrawBody() string {
	return `{"client_id":"c1","product_asset_id":"a1","order_id":"o1","desired_value":"500","if_match":"1"}`
}

// 1. Valid POST with well-formed JSON body delegates to Execute and returns status + serialized response.
func TestWithdrawHandle_ValidPost_DelegatesToExecute(t *testing.T) {
	mock := &mockWithdrawExecutor{
		resp: &application.WithdrawResponse{
			AffectedPositions: []application.AffectedPosition{
				{PositionID: "p1", ClientID: "c1", AssetID: "a1", Amount: "50"},
			},
		},
		statusCode: http.StatusOK,
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if mock.captured.ClientID != "c1" {
		t.Fatalf("expected client_id c1, got %s", mock.captured.ClientID)
	}
	if mock.captured.ProductAssetID != "a1" {
		t.Fatalf("expected product_asset_id a1, got %s", mock.captured.ProductAssetID)
	}
	if mock.captured.DesiredValue != "500" {
		t.Fatalf("expected desired_value 500, got %s", mock.captured.DesiredValue)
	}

	var body application.WithdrawResponse
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}
	if len(body.AffectedPositions) != 1 || body.AffectedPositions[0].PositionID != "p1" {
		t.Fatalf("unexpected response body: %+v", body)
	}
}

// 2. Non-POST methods return 405.
func TestWithdrawHandle_NonPostMethods_Returns405(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	handler := NewWithdrawHandler(&mockWithdrawExecutor{})

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			resp, err := handler.Handle(context.Background(), withdrawRequestWithMethod(method))

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

// 3. POST with malformed JSON body returns 422.
func TestWithdrawHandle_MalformedJSON_Returns422(t *testing.T) {
	handler := NewWithdrawHandler(&mockWithdrawExecutor{})

	resp, err := handler.Handle(context.Background(), withdrawPostRequest("{invalid"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", resp.StatusCode)
	}
	assertErrorBody(t, resp.Body, "invalid request body")
}

// 4. Execute returns (response, 200, nil) → 200 with JSON-serialized WithdrawResponse.
func TestWithdrawHandle_ExecuteReturns200_Success(t *testing.T) {
	mock := &mockWithdrawExecutor{
		resp: &application.WithdrawResponse{
			AffectedPositions: []application.AffectedPosition{
				{PositionID: "p1", Amount: "90"},
			},
		},
		statusCode: http.StatusOK,
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body application.WithdrawResponse
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body.AffectedPositions[0].PositionID != "p1" {
		t.Fatalf("expected position_id p1, got %s", body.AffectedPositions[0].PositionID)
	}
}

// 5. Execute returns (response, 200, nil) for idempotent replay → 200.
func TestWithdrawHandle_ExecuteReturns200_IdempotentReplay(t *testing.T) {
	mock := &mockWithdrawExecutor{
		resp: &application.WithdrawResponse{
			AffectedPositions: []application.AffectedPosition{
				{PositionID: "p2", Amount: "80"},
			},
		},
		statusCode: http.StatusOK,
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body application.WithdrawResponse
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body.AffectedPositions[0].PositionID != "p2" {
		t.Fatalf("expected position_id p2, got %s", body.AffectedPositions[0].PositionID)
	}
}

// 6. Execute returns (nil, 422, error) → 422 with {"error": "..."}.
func TestWithdrawHandle_ExecuteReturns422_ValidationError(t *testing.T) {
	mock := &mockWithdrawExecutor{
		statusCode: http.StatusUnprocessableEntity,
		err:        errors.New("desired_value must be positive"),
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", resp.StatusCode)
	}
	assertErrorBody(t, resp.Body, "desired_value must be positive")
}

// 7. Execute returns (nil, 404, error) → 404 with {"error": "..."}.
func TestWithdrawHandle_ExecuteReturns404_NotFoundError(t *testing.T) {
	mock := &mockWithdrawExecutor{
		statusCode: http.StatusNotFound,
		err:        errors.New("no positions found"),
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
	assertErrorBody(t, resp.Body, "no positions found")
}

// 8. Execute returns (nil, 500, error) → 500 with {"error": "..."}.
func TestWithdrawHandle_ExecuteReturns500_InternalError(t *testing.T) {
	mock := &mockWithdrawExecutor{
		statusCode: http.StatusInternalServerError,
		err:        errors.New("unit of work: connection refused"),
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", resp.StatusCode)
	}
	assertErrorBody(t, resp.Body, "unit of work: connection refused")
}

// 9. Execute returns (nil, 409, plainError) → 409 with {"error": "..."} (no error_code).
func TestWithdrawHandle_ExecuteReturns409_PlainError_NoErrorCode(t *testing.T) {
	mock := &mockWithdrawExecutor{
		statusCode: http.StatusConflict,
		err:        errors.New("replay after race: not found"),
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", resp.StatusCode)
	}
	assertErrorBody(t, resp.Body, "replay after race: not found")

	var parsed map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, exists := parsed["error_code"]; exists {
		t.Fatal("expected no error_code field for plain error, but it was present")
	}
}

// 10. Execute returns (nil, 409, InsufficientPositionError) → 409 with error_code INSUFFICIENT_POSITION.
func TestWithdrawHandle_ExecuteReturns409_InsufficientPositionError(t *testing.T) {
	mock := &mockWithdrawExecutor{
		statusCode: http.StatusConflict,
		err:        &application.InsufficientPositionError{},
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", resp.StatusCode)
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if parsed["error"] != "insufficient position" {
		t.Fatalf("expected error 'insufficient position', got %q", parsed["error"])
	}
	if parsed["error_code"] != "INSUFFICIENT_POSITION" {
		t.Fatalf("expected error_code INSUFFICIENT_POSITION, got %q", parsed["error_code"])
	}
}

// 11. Execute returns (nil, 409, ConcurrencyConflictError) → 409 with error_code CONCURRENCY_CONFLICT.
func TestWithdrawHandle_ExecuteReturns409_ConcurrencyConflictError(t *testing.T) {
	mock := &mockWithdrawExecutor{
		statusCode: http.StatusConflict,
		err:        &application.ConcurrencyConflictError{},
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", resp.StatusCode)
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if parsed["error"] != "concurrency conflict" {
		t.Fatalf("expected error 'concurrency conflict', got %q", parsed["error"])
	}
	if parsed["error_code"] != "CONCURRENCY_CONFLICT" {
		t.Fatalf("expected error_code CONCURRENCY_CONFLICT, got %q", parsed["error_code"])
	}
}

// 12. All responses include Content-Type: application/json header.
func TestWithdrawHandle_AllResponses_HaveContentTypeJSON(t *testing.T) {
	tests := []struct {
		name string
		req  events.APIGatewayV2HTTPRequest
		mock *mockWithdrawExecutor
	}{
		{
			name: "success",
			req:  withdrawPostRequest(validWithdrawBody()),
			mock: &mockWithdrawExecutor{
				resp: &application.WithdrawResponse{
					AffectedPositions: []application.AffectedPosition{{PositionID: "p1"}},
				},
				statusCode: http.StatusOK,
			},
		},
		{
			name: "method not allowed",
			req:  withdrawRequestWithMethod(http.MethodGet),
			mock: &mockWithdrawExecutor{},
		},
		{
			name: "malformed body",
			req:  withdrawPostRequest("{bad"),
			mock: &mockWithdrawExecutor{},
		},
		{
			name: "execute error",
			req:  withdrawPostRequest(validWithdrawBody()),
			mock: &mockWithdrawExecutor{
				statusCode: http.StatusUnprocessableEntity,
				err:        errors.New("validation error"),
			},
		},
		{
			name: "insufficient position error",
			req:  withdrawPostRequest(validWithdrawBody()),
			mock: &mockWithdrawExecutor{
				statusCode: http.StatusConflict,
				err:        &application.InsufficientPositionError{},
			},
		},
		{
			name: "concurrency conflict error",
			req:  withdrawPostRequest(validWithdrawBody()),
			mock: &mockWithdrawExecutor{
				statusCode: http.StatusConflict,
				err:        &application.ConcurrencyConflictError{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewWithdrawHandler(tt.mock)
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

// 13. Verify the handler reads the HTTP method from req.RequestContext.HTTP.Method.
func TestWithdrawHandle_ReadsMethodFromRequestContextHTTP(t *testing.T) {
	handler := NewWithdrawHandler(&mockWithdrawExecutor{})

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
