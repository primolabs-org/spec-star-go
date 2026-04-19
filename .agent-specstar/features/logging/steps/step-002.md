# Step 002 — HTTP handlers: LoggerFactory injection, terminal error logging, bootstrap migration

## Objective

Inject `LoggerFactory` into HTTP handlers, add terminal error logging to `Handle` methods, store the enriched logger in context for downstream consumers, and migrate bootstrap startup errors from `log.Fatalf` to `slog`.

## In Scope

- Define consumer-side `loggerFactory` interface in `httphandler` package:
  ```go
  type loggerFactory interface {
      FromContext(ctx context.Context, trigger, operation string) *slog.Logger
  }
  ```
- Add `loggers loggerFactory` field to `DepositHandler` and `WithdrawHandler`.
- Update `NewDepositHandler` and `NewWithdrawHandler` constructors to accept `loggerFactory`.
- In each `Handle` method:
  1. Call `h.loggers.FromContext(ctx, "http", "<operation>")` to get per-request logger.
  2. Call `platform.WithLogger(ctx, logger)` to store enriched logger in context.
  3. Pass enriched `ctx` to `h.service.Execute(ctx, req)`.
  4. On `Execute` error: log at ERROR (status >= 500) or WARN (status < 500) with `status`, `error`, `outcome=failed` fields.
- In `cmd/http-lambda/main.go`:
  1. Create `LoggerFactory` before config loading.
  2. Replace `log.Fatalf` calls with `slog.Error(...)` + `os.Exit(1)`.
  3. Pass `LoggerFactory` to handler constructors.
  4. Remove `"log"` import, add `"log/slog"`, `"os"`, and `platform` imports.
- Update handler tests:
  1. Define `mockLoggerFactory` in test files.
  2. Update constructor calls to pass mock factory.
  3. Add test cases verifying log output for terminal error paths.

## Out of Scope

- Logging body parse failures or method-not-allowed responses (trivial HTTP boundary issues).
- Logging successful responses.
- Modifying application services, outbound adapters, domain, or ports.
- Changing the `depositExecutor` / `withdrawExecutor` interfaces.
- Adding logging to the Lambda routing switch in `main.go`.

## Required Reads

- `internal/adapters/inbound/httphandler/deposit_handler.go`
- `internal/adapters/inbound/httphandler/withdraw_handler.go`
- `internal/adapters/inbound/httphandler/deposit_handler_test.go`
- `internal/adapters/inbound/httphandler/withdraw_handler_test.go`
- `cmd/http-lambda/main.go`
- `internal/platform/logger.go` (created in step-001)
- `.agent-specstar/features/logging/design.md` — Technical Architecture section.

## Allowed Write Paths

- `internal/adapters/inbound/httphandler/deposit_handler.go` (MODIFY)
- `internal/adapters/inbound/httphandler/withdraw_handler.go` (MODIFY)
- `internal/adapters/inbound/httphandler/deposit_handler_test.go` (MODIFY)
- `internal/adapters/inbound/httphandler/withdraw_handler_test.go` (MODIFY)
- `cmd/http-lambda/main.go` (MODIFY)

## Forbidden Paths

- `internal/application/`
- `internal/adapters/outbound/`
- `internal/domain/`
- `internal/ports/`
- `internal/platform/logger.go` (read only, created in step-001)
- `internal/platform/database.go`

## Known Abstraction Opportunities

- The `loggerFactory` interface is the consumer-defined abstraction for testability. No further abstraction needed.

## Allowed Abstraction Scope

- `loggerFactory` interface in `httphandler` package only.

## Required Tests

**deposit_handler_test.go:**
1. Existing tests updated with mock `loggerFactory` — all continue to pass.
2. New test: service Execute returns 5xx error → handler logs at ERROR level with `status`, `error`, `outcome` fields.
3. New test: service Execute returns 4xx error → handler logs at WARN level with same fields.
4. Verify that context passed to service Execute contains the enriched logger (via mock that captures context).

**withdraw_handler_test.go:**
1. Same pattern as deposit handler tests.
2. Verify logging for `ErrInsufficientPosition` (4xx → WARN) and `ErrConcurrencyConflict` (4xx → WARN) paths.
3. Verify logging for 5xx service error paths.

## Coverage Requirement

100% on all changed lines in:
- `deposit_handler.go`
- `withdraw_handler.go`

`main.go` is bootstrap code — coverage is not enforced via unit tests. Bootstrap correctness is validated by integration/e2e tests.

## Failure Model

- Handler logging does not affect request processing. If `FromContext` returns a logger (it always does), logging calls are fire-and-forget. The request response is unchanged.
- `platform.WithLogger` is a pure context operation — cannot fail.

## Allowed Fallbacks

- None. All paths are deterministic.

## Acceptance Criteria

1. `go test ./internal/adapters/inbound/httphandler/...` passes with 100% coverage on changed lines.
2. `go build ./cmd/http-lambda/` compiles without errors.
3. `main.go` no longer imports `"log"`. Uses `"log/slog"` and `"os"` for startup errors.
4. Handler constructors require a `loggerFactory` parameter. Compilation fails without it.
5. Service Execute errors produce structured log entries with `status`, `error`, `outcome` fields.
6. Enriched context (with logger) is passed to `service.Execute`.
7. All existing tests in the repository continue to pass.

## Deferred Work

- None.

## Escalation Conditions

- If the `loggerFactory` interface causes import cycles, escalate to determine package placement.
- If handler test mock setup becomes excessively verbose, consider a shared test helper (but prefer inline mock first).
