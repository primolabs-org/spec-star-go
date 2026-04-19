# Step 003 — Inbound HTTP handler adapter

## Objective

Implement a thin API Gateway HTTP API v2 handler in `internal/adapters/inbound/httphandler/` that maps Lambda events to/from `DepositService` and tests all mapping paths.

## In Scope

- `DepositHandler` struct with constructor accepting `*application.DepositService`.
- `Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)` method.
- Route `POST` requests to `DepositService.Execute`. Return `405 Method Not Allowed` for all other HTTP methods.
- Parse `req.Body` as `DepositRequest` (JSON unmarshal). If unmarshal fails, return `422` with `{"error": "invalid request body"}`.
- On success: return the status code from `Execute`, with `DepositResponse` serialized as JSON body and `Content-Type: application/json`.
- On error from `Execute`: return the status code from `Execute`, with `{"error": "<message>"}` as JSON body and `Content-Type: application/json`.
- The handler returns `(response, nil)` for all expected outcomes — never returns an error from the Lambda function itself for business/validation cases. Only returns a non-nil error for truly unrecoverable scenarios (e.g., response serialization failure). This ensures API Gateway does not generate a generic 5xx for expected failures.
- Unit tests covering all mapping paths.

## Out of Scope

- Business logic, validation, or orchestration (owned by `DepositService` in step 002).
- Lambda bootstrap wiring (step 004).
- Structured logging.

## Required Reads

- `internal/application/deposit_service.go` — `DepositService`, `DepositRequest`, `DepositResponse` types and `Execute` signature (from step 002).
- `github.com/aws/aws-lambda-go/events` — `APIGatewayV2HTTPRequest` and `APIGatewayV2HTTPResponse` types.

## Allowed Write Paths

- `internal/adapters/inbound/httphandler/deposit_handler.go`
- `internal/adapters/inbound/httphandler/deposit_handler_test.go`

## Forbidden Paths

- `internal/application/` — application layer is complete from step 002.
- `internal/domain/` — domain model is stable.
- `internal/ports/` — port interfaces are stable.
- `cmd/` — bootstrap belongs to step 004.

## Known Abstraction Opportunities

- A private `jsonResponse(statusCode int, body any) events.APIGatewayV2HTTPResponse` helper to reduce mapping duplication between success and error response construction.
- A private `errorResponse(statusCode int, message string) events.APIGatewayV2HTTPResponse` helper for error JSON.

## Allowed Abstraction Scope

- Private helpers within `deposit_handler.go` only. No new packages, no new interfaces.

## Required Tests

Tests use the handler's `Handle` method directly with constructed `APIGatewayV2HTTPRequest` values. No need for real HTTP servers or Lambda runtime.

### Request mapping
1. Valid POST with well-formed JSON body: handler delegates to `DepositService.Execute`, returns the status code and serialized response.
2. Non-POST method (GET, PUT, DELETE): returns `405` with `{"error": "method not allowed"}`.
3. POST with malformed JSON body: returns `422` with `{"error": "invalid request body"}`.

### Response mapping
4. `Execute` returns `(response, 201, nil)` → handler returns `201` with JSON-serialized `DepositResponse`.
5. `Execute` returns `(response, 200, nil)` → handler returns `200` with JSON-serialized `DepositResponse` (idempotent replay).
6. `Execute` returns `(nil, 422, error)` → handler returns `422` with error JSON.
7. `Execute` returns `(nil, 409, error)` → handler returns `409` with error JSON.
8. `Execute` returns `(nil, 500, error)` → handler returns `500` with error JSON.

### Headers
9. All responses include `Content-Type: application/json` header.

### HTTP method extraction
10. Verify the handler reads the HTTP method from `req.RequestContext.HTTP.Method` (API Gateway HTTP API v2 event format).

## Coverage Requirement

100% line coverage on `internal/adapters/inbound/httphandler/deposit_handler.go`.

## Failure Model

- Malformed JSON body → `422` response (not a Lambda invocation error).
- Any `DepositService.Execute` error → mapped to the returned status code as an API Gateway response (not a Lambda invocation error).
- Response serialization failure (marshalling `DepositResponse`) → return the error from the Lambda function. This is an unrecoverable programmer bug, not a business error.

## Allowed Fallbacks

None.

## Acceptance Criteria

1. `DepositHandler.Handle` correctly delegates to `DepositService.Execute` for POST requests.
2. Non-POST methods return `405`.
3. Malformed JSON returns `422`.
4. All responses are valid JSON with `Content-Type: application/json`.
5. The handler never returns a non-nil error for expected business/validation outcomes.
6. All test scenarios pass.
7. `go build ./...` succeeds.
8. `go vet ./...` produces no warnings on new files.

## Deferred Work

None.

## Escalation Conditions

- If `events.APIGatewayV2HTTPRequest` HTTP method field is not at `RequestContext.HTTP.Method`, verify against the `aws-lambda-go` library source before proceeding.
- If the handler needs to read path parameters or query strings for routing, escalate — the current design assumes a single route.
