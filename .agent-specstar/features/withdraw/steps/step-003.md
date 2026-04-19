# Step 003 - Implement WithdrawHandler

## Metadata
- Feature: withdraw
- Step: step-003
- Status: pending
- Depends On: step-002
- Last Updated: 2026-04-19

## Objective

Implement `WithdrawHandler` in the HTTP adapter layer that maps API Gateway HTTP API v2 events to `WithdrawService`, including error-code mapping for 409 responses that carry a `code` field (`INSUFFICIENT_POSITION`, `CONCURRENCY_CONFLICT`).

## In Scope

- New `internal/adapters/inbound/httphandler/withdraw_handler.go` containing:
  - `withdrawExecutor` interface matching `WithdrawService.Execute` signature.
  - `WithdrawHandler` struct with constructor `NewWithdrawHandler`.
  - `Handle(ctx, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)` method.
  - `codedErrorResponse` package-level helper for 409 responses with `{"error": "message", "code": "CODE"}` shape.
- New `internal/adapters/inbound/httphandler/withdraw_handler_test.go` with all unit tests.

### Handler Flow

1. Check `req.RequestContext.HTTP.Method == POST`. If not → `405 Method Not Allowed`.
2. Unmarshal `req.Body` into `application.WithdrawRequest`. If fails → `422 Unprocessable Entity` with `"invalid request body"`.
3. Call `service.Execute(ctx, req)`.
4. If `err != nil`:
   - If `errors.Is(err, domain.ErrInsufficientPosition)` → `codedErrorResponse(409, err.Error(), "INSUFFICIENT_POSITION")`.
   - If `errors.Is(err, domain.ErrConcurrencyConflict)` → `codedErrorResponse(409, err.Error(), "CONCURRENCY_CONFLICT")`.
   - Otherwise → `errorResponse(statusCode, err.Error())` (existing helper).
5. If `err == nil` → `jsonResponse(statusCode, resp)` (existing helper).

### Error Response Helpers

- `errorResponse` already exists in `deposit_handler.go` (package-level, reusable).
- `jsonResponse` already exists in `deposit_handler.go` (package-level, reusable).
- New: `codedErrorResponse(statusCode int, message, code string)` returns `jsonResponse(statusCode, map[string]string{"error": message, "code": code})`.

## Out of Scope

- Lambda routing changes in `main.go`.
- Any changes to `WithdrawService`, domain, or ports.
- Changes to `deposit_handler.go`.

## Required Reads

- `internal/adapters/inbound/httphandler/deposit_handler.go` — reference pattern for handler structure, `jsonResponse`, `errorResponse`, `jsonMarshal` test seam, `depositExecutor` interface.
- `internal/adapters/inbound/httphandler/deposit_handler_test.go` — reference pattern for mock executor, request builders, assertion helpers.
- `internal/application/withdraw_service.go` — `WithdrawRequest`, `WithdrawResponse` types, `Execute` signature.
- `internal/domain/errors.go` — `ErrInsufficientPosition`, `ErrConcurrencyConflict` for `errors.Is` checks.

## Allowed Write Paths

- `internal/adapters/inbound/httphandler/withdraw_handler.go`
- `internal/adapters/inbound/httphandler/withdraw_handler_test.go`

## Forbidden Paths

- `internal/adapters/inbound/httphandler/deposit_handler.go`
- `internal/adapters/inbound/httphandler/deposit_handler_test.go`
- `internal/application/`
- `internal/domain/`
- `internal/ports/`
- `cmd/`

## Known Abstraction Opportunities

None.

## Allowed Abstraction Scope

- `codedErrorResponse` as a package-level helper in `withdraw_handler.go`.

## Required Tests

All tests use a `mockWithdrawExecutor` struct implementing the `withdrawExecutor` interface.

1. **Successful request → 200**: mock returns `*WithdrawResponse` with status 200, verify response status code 200, verify JSON body deserializes to `WithdrawResponse`, verify `Content-Type: application/json`.
2. **Non-POST methods → 405**: test GET, PUT, DELETE, PATCH, verify status 405, verify error body `"method not allowed"`.
3. **Invalid JSON body → 422**: send malformed JSON, verify status 422, verify error body `"invalid request body"`.
4. **Service returns 422 validation error**: mock returns `(nil, 422, error)`, verify status 422, verify error message forwarded.
5. **Service returns 404 not found**: mock returns `(nil, 404, error)`, verify status 404, verify error body without code field.
6. **Service returns 409 with ErrInsufficientPosition**: mock returns `(nil, 409, fmt.Errorf("...: %w", domain.ErrInsufficientPosition))`, verify status 409, verify body has `"code": "INSUFFICIENT_POSITION"`.
7. **Service returns 409 with ErrConcurrencyConflict**: mock returns `(nil, 409, fmt.Errorf("...: %w", domain.ErrConcurrencyConflict))`, verify status 409, verify body has `"code": "CONCURRENCY_CONFLICT"`.
8. **Service returns 409 without recognized sentinel (race replay failure)**: mock returns `(nil, 409, errors.New("replay after race: ..."))`, verify status 409, verify body uses plain error format (no code field).
9. **Service returns 500 internal error**: mock returns `(nil, 500, error)`, verify status 500, verify error message forwarded.
10. **All responses have Content-Type JSON header**: verify across success, 405, 422, and error scenarios.

## Coverage Requirement

100% on all lines in `withdraw_handler.go`.

## Failure Model

- Non-POST method → 405, no service call.
- Invalid JSON → 422, no service call.
- Service error → mapped to correct status code and body shape at the handler boundary.
- `jsonMarshal` failure → propagated as `(empty, error)` pair (same as deposit handler).

## Allowed Fallbacks

None.

## Acceptance Criteria

1. `WithdrawHandler.Handle` correctly routes POST requests to the service and maps all other methods to 405.
2. Invalid JSON bodies produce 422 before reaching the service.
3. 409 responses with `ErrInsufficientPosition` include `"code": "INSUFFICIENT_POSITION"` in the body.
4. 409 responses with `ErrConcurrencyConflict` include `"code": "CONCURRENCY_CONFLICT"` in the body.
5. 409 responses without a recognized sentinel use the plain error format (no code field).
6. All other error status codes use `{"error": "message"}` format.
7. `jsonResponse` and `errorResponse` from `deposit_handler.go` are reused without modification.
8. All 10 test cases pass.
9. 100% coverage on `withdraw_handler.go`.
10. No changes to existing files.

## Deferred Work

None.

## Escalation Conditions

- If `jsonResponse` or `errorResponse` in `deposit_handler.go` are unexpectedly not accessible from `withdraw_handler.go` (e.g., due to build tags or test-only visibility), escalate.
