# Step 003 Fix 001 — Propagate json.Marshal error in jsonResponse

## Problem

`jsonResponse` in `deposit_handler.go` discards the `json.Marshal` error with `_`. The step 003 Failure Model explicitly requires serialization failures to be returned as the Lambda function error.

## Scope

- `internal/adapters/inbound/httphandler/deposit_handler.go`
- `internal/adapters/inbound/httphandler/deposit_handler_test.go`

## Required Changes

### deposit_handler.go

1. Change `jsonResponse` to return `(events.APIGatewayV2HTTPResponse, error)`. Handle the `json.Marshal` error by returning it.
2. Update `errorResponse` to return `(events.APIGatewayV2HTTPResponse, error)` accordingly.
3. Update `Handle` to propagate errors from `jsonResponse` and `errorResponse`. For expected error responses (405, 422, Execute errors), the marshal target is always `map[string]string` which cannot fail — but the signature must still propagate to satisfy the contract for `jsonResponse` when called with `DepositResponse` on the success path.

### deposit_handler_test.go

4. Add a test that exercises the `json.Marshal` failure path in `jsonResponse` to maintain 100% line coverage. Pass an unmarshalable value (e.g., via a type that causes `json.Marshal` to fail) and verify the handler returns a non-nil error.

## Constraints

- No new packages, no new exported types.
- Keep the handler thin.
- Maintain 100% line coverage on `deposit_handler.go`.

## Acceptance Criteria

1. `json.Marshal` error in `jsonResponse` is propagated, not discarded.
2. `Handle` returns `(response, err)` when serialization fails.
3. All existing tests continue to pass.
4. New test covers the serialization failure path.
5. 100% line coverage on `deposit_handler.go`.
