# Step 002 Review — Inbound HTTP handler adapter: WithdrawHandler

## Verdict: ✅ APPROVED

## Files Reviewed

| File | Status |
|---|---|
| `internal/adapters/inbound/httphandler/withdraw_handler.go` | New — 59 lines |
| `internal/adapters/inbound/httphandler/withdraw_handler_test.go` | New — 417 lines |

## Scope Compliance

**Allowed write paths**: Only the two files above were created. Both are within the allowed paths specified in the step file. No forbidden paths (`internal/application/`, `internal/domain/`, `internal/ports/`, `deposit_handler.go`, `cmd/`) were modified.

**No scope creep**: The implementation is limited to exactly what the step specifies — a thin HTTP handler adapter with typed error mapping and a private `codedErrorResponse` helper.

## Acceptance Criteria Verification

| # | Criterion | Status | Evidence |
|---|---|---|---|
| 1 | `Handle` delegates to `withdrawExecutor.Execute` for POST | ✅ | Line 38: `h.service.Execute(ctx, withdrawReq)` after POST check and JSON unmarshal |
| 2 | Non-POST methods return 405 | ✅ | Lines 29-31: `errorResponse(http.StatusMethodNotAllowed, "method not allowed")` |
| 3 | Malformed JSON returns 422 | ✅ | Lines 33-36: `errorResponse(http.StatusUnprocessableEntity, "invalid request body")` |
| 4 | `InsufficientPositionError` → 409 + `INSUFFICIENT_POSITION` | ✅ | Lines 40-43: `errors.As` check → `codedErrorResponse(statusCode, err.Error(), "INSUFFICIENT_POSITION")` |
| 5 | `ConcurrencyConflictError` → 409 + `CONCURRENCY_CONFLICT` | ✅ | Lines 45-48: `errors.As` check → `codedErrorResponse(statusCode, err.Error(), "CONCURRENCY_CONFLICT")` |
| 6 | Plain 409 → `{"error": "..."}` without `error_code` | ✅ | Line 50: falls through to `errorResponse(statusCode, err.Error())` |
| 7 | All responses are valid JSON with `Content-Type: application/json` | ✅ | All paths route through `jsonResponse` (directly or via `errorResponse`/`codedErrorResponse`); `jsonResponse` sets the header |
| 8 | Handler never returns non-nil error for business/validation outcomes | ✅ | All business paths return `(response, nil)` via helpers; only `jsonResponse` marshal failure can return error (unrecoverable bug, per spec) |
| 9 | All 13 test scenarios pass | ✅ | Confirmed via `go test -v -run Withdraw` — 13 top-level tests, all PASS |
| 10 | `go build ./...` succeeds | ✅ | Verified — exit 0 |
| 11 | `go vet ./...` — no warnings | ✅ | Verified — exit 0 |

## Test Coverage

```
withdraw_handler.go:23  NewWithdrawHandler  100.0%
withdraw_handler.go:28  Handle              100.0%
withdraw_handler.go:56  codedErrorResponse  100.0%
```

**100% line coverage on `withdraw_handler.go`** — meets the step requirement.

## Test Scenario Mapping

| # | Required Test | Test Function | Status |
|---|---|---|---|
| 1 | Valid POST delegates to Execute | `TestWithdrawHandle_ValidPost_DelegatesToExecute` | ✅ |
| 2 | Non-POST returns 405 | `TestWithdrawHandle_NonPostMethods_Returns405` (GET, PUT, DELETE subtests) | ✅ |
| 3 | Malformed JSON returns 422 | `TestWithdrawHandle_MalformedJSON_Returns422` | ✅ |
| 4 | Execute → 200 success | `TestWithdrawHandle_ExecuteReturns200_Success` | ✅ |
| 5 | Execute → 200 idempotent replay | `TestWithdrawHandle_ExecuteReturns200_IdempotentReplay` | ✅ |
| 6 | Execute → 422 validation error | `TestWithdrawHandle_ExecuteReturns422_ValidationError` | ✅ |
| 7 | Execute → 404 not found | `TestWithdrawHandle_ExecuteReturns404_NotFoundError` | ✅ |
| 8 | Execute → 500 internal error | `TestWithdrawHandle_ExecuteReturns500_InternalError` | ✅ |
| 9 | Execute → 409 plain error (no error_code) | `TestWithdrawHandle_ExecuteReturns409_PlainError_NoErrorCode` | ✅ |
| 10 | Execute → 409 InsufficientPositionError | `TestWithdrawHandle_ExecuteReturns409_InsufficientPositionError` | ✅ |
| 11 | Execute → 409 ConcurrencyConflictError | `TestWithdrawHandle_ExecuteReturns409_ConcurrencyConflictError` | ✅ |
| 12 | All responses have Content-Type header | `TestWithdrawHandle_AllResponses_HaveContentTypeJSON` (6 subtests) | ✅ |
| 13 | Method read from RequestContext.HTTP.Method | `TestWithdrawHandle_ReadsMethodFromRequestContextHTTP` | ✅ |

## Pattern Compliance (deposit_handler.go alignment)

| Aspect | deposit_handler.go | withdraw_handler.go | Match |
|---|---|---|---|
| Interface pattern | `depositExecutor` | `withdrawExecutor` | ✅ |
| Struct with `service` field | `DepositHandler{service}` | `WithdrawHandler{service}` | ✅ |
| Constructor | `NewDepositHandler(depositExecutor)` | `NewWithdrawHandler(withdrawExecutor)` | ✅ |
| Handle signature | `(ctx, req) → (resp, error)` | `(ctx, req) → (resp, error)` | ✅ |
| Method check → unmarshal → execute flow | ✅ | ✅ | ✅ |
| Reuses `jsonResponse` | — (defines it) | ✅ (calls it) | ✅ |
| Reuses `errorResponse` | — (defines it) | ✅ (calls it) | ✅ |
| Reuses `jsonMarshal` test seam | — (defines it) | ✅ (via `jsonResponse`) | ✅ |

## Abstraction Decisions

- `codedErrorResponse` (lines 56-58): Private helper producing `{"error": "...", "error_code": "..."}` via `jsonResponse`. Matches the step specification exactly. No over-abstraction.

## Clean Code Check

- No dead code or stale comments.
- No silent fallbacks or guessed defaults.
- No duplicated logic — typed error mapping is unique to withdraw; shared helpers are reused.
- Imports are minimal and all used.
- Naming follows Go conventions and matches the deposit handler pattern.

## Future-Step Awareness

Step 003 (Lambda bootstrap routing) depends on `NewWithdrawHandler` existing — this is now satisfied. No deferred work or temporary breakage introduced.

## Issues Found

None.
