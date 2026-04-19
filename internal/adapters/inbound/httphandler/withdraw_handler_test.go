package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/primolabs-org/spec-star-go/internal/application"
	"github.com/primolabs-org/spec-star-go/internal/domain"
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
	return `{"client_id":"c1","instrument_id":"i1","order_id":"o1","desired_value":"500"}`
}

func TestWithdrawHandle_SuccessfulRequest_Returns200(t *testing.T) {
	mock := &mockWithdrawExecutor{
		resp: &application.WithdrawResponse{
			Positions: []application.PositionDTO{
				{
					PositionID: "p1",
					ClientID:   "c1",
					AssetID:    "a1",
					Amount:     "50",
					UnitPrice:  "10",
					TotalValue: "500",
				},
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
	if resp.Headers["Content-Type"] != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", resp.Headers["Content-Type"])
	}

	var body application.WithdrawResponse
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}
	if len(body.Positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(body.Positions))
	}
	if body.Positions[0].PositionID != "p1" {
		t.Fatalf("expected position_id p1, got %s", body.Positions[0].PositionID)
	}
}

func TestWithdrawHandle_NonPostMethods_Returns405(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
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

func TestWithdrawHandle_InvalidJSONBody_Returns422(t *testing.T) {
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

func TestWithdrawHandle_ServiceReturns422_ValidationError(t *testing.T) {
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

func TestWithdrawHandle_ServiceReturns404_NotFound(t *testing.T) {
	mock := &mockWithdrawExecutor{
		statusCode: http.StatusNotFound,
		err:        errors.New("client not found"),
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal error body: %v", err)
	}
	if body["error"] != "client not found" {
		t.Fatalf("expected error 'client not found', got %q", body["error"])
	}
	if _, hasCode := body["code"]; hasCode {
		t.Fatal("expected no 'code' field in 404 response")
	}
}

func TestWithdrawHandle_ServiceReturns409_ErrInsufficientPosition(t *testing.T) {
	mock := &mockWithdrawExecutor{
		statusCode: http.StatusConflict,
		err:        fmt.Errorf("total available 100 less than desired 500: %w", domain.ErrInsufficientPosition),
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal error body: %v", err)
	}
	if body["code"] != "INSUFFICIENT_POSITION" {
		t.Fatalf("expected code INSUFFICIENT_POSITION, got %q", body["code"])
	}
	if body["error"] != mock.err.Error() {
		t.Fatalf("expected error %q, got %q", mock.err.Error(), body["error"])
	}
}

func TestWithdrawHandle_ServiceReturns409_ErrConcurrencyConflict(t *testing.T) {
	mock := &mockWithdrawExecutor{
		statusCode: http.StatusConflict,
		err:        fmt.Errorf("position version mismatch: %w", domain.ErrConcurrencyConflict),
	}
	handler := NewWithdrawHandler(mock)

	resp, err := handler.Handle(context.Background(), withdrawPostRequest(validWithdrawBody()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal error body: %v", err)
	}
	if body["code"] != "CONCURRENCY_CONFLICT" {
		t.Fatalf("expected code CONCURRENCY_CONFLICT, got %q", body["code"])
	}
	if body["error"] != mock.err.Error() {
		t.Fatalf("expected error %q, got %q", mock.err.Error(), body["error"])
	}
}

func TestWithdrawHandle_ServiceReturns409_NoRecognizedSentinel(t *testing.T) {
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

	var body map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("failed to unmarshal error body: %v", err)
	}
	if body["error"] != "replay after race: not found" {
		t.Fatalf("expected error 'replay after race: not found', got %q", body["error"])
	}
	if _, hasCode := body["code"]; hasCode {
		t.Fatal("expected no 'code' field for unrecognized 409")
	}
}

func TestWithdrawHandle_ServiceReturns500_InternalError(t *testing.T) {
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
				resp:       &application.WithdrawResponse{Positions: []application.PositionDTO{{PositionID: "p1"}}},
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
			name: "service error",
			req:  withdrawPostRequest(validWithdrawBody()),
			mock: &mockWithdrawExecutor{
				statusCode: http.StatusUnprocessableEntity,
				err:        errors.New("validation error"),
			},
		},
		{
			name: "coded error",
			req:  withdrawPostRequest(validWithdrawBody()),
			mock: &mockWithdrawExecutor{
				statusCode: http.StatusConflict,
				err:        fmt.Errorf("insufficient: %w", domain.ErrInsufficientPosition),
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
