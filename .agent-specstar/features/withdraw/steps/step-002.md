# Step 002 — Inbound HTTP handler adapter: WithdrawHandler

## Objective

Implement a thin API Gateway HTTP API v2 handler in `internal/adapters/inbound/httphandler/` that maps Lambda events to/from `WithdrawService`, supports structured `error_code` in 409 responses, and tests all mapping paths.

## Dependencies

- Step 001 must be complete (`WithdrawService`, `WithdrawRequest`, `WithdrawResponse`, `InsufficientPositionError`, `ConcurrencyConflictError` must exist).

## In Scope

- `withdrawExecutor` interface in `withdraw_handler.go` with method `Execute(ctx context.Context, req application.WithdrawRequest) (*application.WithdrawResponse, int, error)` — same pattern as `depositExecutor`.
- `WithdrawHandler` struct with constructor accepting `withdrawExecutor`.
- `Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)` method.
- Route `POST` requests to `WithdrawService.Execute`. Return `405 Method Not Allowed` for all other HTTP methods.
- Parse `req.Body` as `WithdrawRequest` (JSON unmarshal). If unmarshal fails, return `422` with `{"error": "invalid request body"}`.
- On success: return the status code from `Execute`, with `WithdrawResponse` serialized as JSON body and `Content-Type: application/json`.
- On error from `Execute`: inspect typed errors to determine response shape:
  - `InsufficientPositionError` → `codedErrorResponse(statusCode, message, "INSUFFICIENT_POSITION")`
  - `ConcurrencyConflictError` → `codedErrorResponse(statusCode, message, "CONCURRENCY_CONFLICT")`
  - All other errors → `errorResponse(statusCode, message)` (reuse existing helper from `deposit_handler.go`)
- Private `codedErrorResponse(statusCode int, message, errorCode string)` helper that produces `{"error": "...", "error_code": "..."}`.
- Reuse existing `jsonResponse` and `errorResponse` helpers from `deposit_handler.go` (same package).
- Reuse existing `jsonMarshal` test seam variable from `deposit_handler.go` (same package).
- The handler returns `(response, nil)` for all expected outcomes — never returns an error from the Lambda function itself for business/validation cases.
- Unit tests covering all mapping paths.

## Out of Scope

- Business logic, validation, lot selection, or orchestration (owned by `WithdrawService` in step 001).
- Lambda bootstrap wiring (step 003).
- Structured logging.

## Required Reads

- `internal/application/withdraw_service.go` — `WithdrawService`, `WithdrawRequest`, `WithdrawResponse`, `InsufficientPositionError`, `ConcurrencyConflictError` types and `Execute` signature (from step 001).
- `internal/adapters/inbound/httphandler/deposit_handler.go` — `depositExecutor` interface pattern, `jsonResponse`, `errorResponse`, `jsonMarshal` variable.
- `internal/adapters/inbound/httphandler/deposit_handler_test.go` — mock executor pattern, test request helpers.
- `github.com/aws/aws-lambda-go/events` — `APIGatewayV2HTTPRequest` and `APIGatewayV2HTTPResponse` types.

## Allowed Write Paths

- `internal/adapters/inbound/httphandler/withdraw_handler.go`
- `internal/adapters/inbound/httphandler/withdraw_handler_test.go`

## Forbidden Paths

- `internal/application/` — application layer is complete from step 001.
- `internal/domain/` — domain model is stable.
- `internal/ports/` — port interfaces are stable.
- `internal/adapters/inbound/httphandler/deposit_handler.go` — existing handler is not modified.
- `cmd/` — bootstrap belongs to step 003.

## Known Abstraction Opportunities

- `codedErrorResponse` private helper for `{"error": "...", "error_code": "..."}` shape.

## Allowed Abstraction Scope

- Private helpers within `withdraw_handler.go` only. No new packages.

## Required Tests

Tests use the handler's `Handle` method directly with constructed `APIGatewayV2HTTPRequest` values. The mock `withdrawExecutor` captures the request and returns configured responses/errors.

### Request mapping
1. Valid POST with well-formed JSON body: handler delegates to `WithdrawService.Execute`, returns the status code and serialized response.
2. Non-POST method (GET, PUT, DELETE): returns `405` with `{"error": "method not allowed"}`.
3. POST with malformed JSON body: returns `422` with `{"error": "invalid request body"}`.

### Response mapping — success
4. `Execute` returns `(response, 200, nil)` → handler returns `200` with JSON-serialized `WithdrawResponse`.
5. `Execute` returns `(response, 200, nil)` for idempotent replay → handler returns `200` with JSON-serialized `WithdrawResponse`.

### Response mapping — errors without error_code
6. `Execute` returns `(nil, 422, error)` → handler returns `422` with `{"error": "..."}`.
7. `Execute` returns `(nil, 404, error)` → handler returns `404` with `{"error": "..."}`.
8. `Execute` returns `(nil, 500, error)` → handler returns `500` with `{"error": "..."}`.
9. `Execute` returns `(nil, 409, plainError)` (not typed) → handler returns `409` with `{"error": "..."}` (no `error_code` field).

### Response mapping — errors with error_code
10. `Execute` returns `(nil, 409, InsufficientPositionError)` → handler returns `409` with `{"error": "insufficient position", "error_code": "INSUFFICIENT_POSITION"}`.
11. `Execute` returns `(nil, 409, ConcurrencyConflictError)` → handler returns `409` with `{"error": "concurrency conflict", "error_code": "CONCURRENCY_CONFLICT"}`.

### Headers
12. All responses include `Content-Type: application/json` header.

### HTTP method extraction
13. Verify the handler reads the HTTP method from `req.RequestContext.HTTP.Method` (API Gateway HTTP API v2 event format).

## Coverage Requirement

100% line coverage on `internal/adapters/inbound/httphandler/withdraw_handler.go`.

## Failure Model

- Malformed JSON body → `422` response (not a Lambda invocation error).
- Any `WithdrawService.Execute` error → mapped to the returned status code as an API Gateway response (not a Lambda invocation error).
- `InsufficientPositionError` and `ConcurrencyConflictError` → `409` with `error_code` in response body.
- Response serialization failure (marshalling `WithdrawResponse`) → return the error from the Lambda function. This is an unrecoverable programmer bug, not a business error.

## Allowed Fallbacks

None.

## Acceptance Criteria

1. `WithdrawHandler.Handle` correctly delegates to `withdrawExecutor.Execute` for POST requests.
2. Non-POST methods return `405`.
3. Malformed JSON returns `422`.
4. `InsufficientPositionError` produces `{"error": "...", "error_code": "INSUFFICIENT_POSITION"}` with status `409`.
5. `ConcurrencyConflictError` produces `{"error": "...", "error_code": "CONCURRENCY_CONFLICT"}` with status `409`.
6. Plain 409 errors (no typed error) produce `{"error": "..."}` without `error_code`.
7. All responses are valid JSON with `Content-Type: application/json`.
8. The handler never returns a non-nil error for expected business/validation outcomes.
9. All test scenarios pass.
10. `go build ./...` succeeds.
11. `go vet ./...` produces no warnings on new files.

## Deferred Work

None.

## Escalation Conditions

- If `errors.As` cannot inspect the typed errors from step 001 (e.g., they are pointer receivers but returned as values or vice versa), verify the error type definitions before proceeding.
- If the existing `jsonResponse` or `errorResponse` helpers are not callable from `withdraw_handler.go` (e.g., unexported or wrong package), verify — they should be accessible since both files are in the same `httphandler` package.
