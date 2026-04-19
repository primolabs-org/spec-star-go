# Step 003 — Application services: operation outcome and idempotency logging

## Objective

Add structured logging to application services for terminal infrastructure failures and idempotency replay events using the correlated logger from context.

## In Scope

- `DepositService.Execute`:
  1. Retrieve logger via `platform.LoggerFromContext(ctx)`.
  2. Log idempotency replay (existing processed command found) at INFO with `order_id`, `outcome=replayed`.
  3. Log race condition replay (`ErrDuplicate` → `replayAfterRace`) at INFO with `order_id`, `outcome=replayed`.
  4. Log terminal infrastructure failures (5xx paths: failed to query processed command, failed to find client due to infra error, failed to find asset due to infra error, unit of work failure) at ERROR with `error`, `outcome=failed`, and relevant entity identifiers (`client_id`, `order_id`, `asset_id` where available).
  5. Do NOT log 4xx validation failures (client not found, asset not found, invalid product type) — these are expected business outcomes.

- `WithdrawService.Execute`:
  1. Same logger retrieval pattern.
  2. Log idempotency replay at INFO with `order_id`, `outcome=replayed`.
  3. Log race condition replay at INFO with `order_id`, `outcome=replayed`.
  4. Log terminal infrastructure failures (5xx paths: failed to query processed command, unit of work unexpected failure) at ERROR.
  5. Do NOT log 4xx business outcomes (ErrNotFound, ErrInsufficientPosition, ErrConcurrencyConflict) — these are handled by the handler's WARN logging.

- Update service tests to verify logging behavior.

## Out of Scope

- Changing service constructor signatures. Logger comes from context, not constructor injection.
- Logging in validation functions (`validateDepositRequest`, `validateWithdrawRequest`).
- Logging in snapshot serialization/deserialization helpers.
- Modifying handler, adapter, domain, or port code.
- Changing `fmt.Errorf` calls — these are error wrapping, not logging.

## Required Reads

- `internal/application/deposit_service.go`
- `internal/application/withdraw_service.go`
- `internal/application/deposit_service_test.go`
- `internal/application/withdraw_service_test.go`
- `internal/platform/logger.go` (created in step-001) — for `LoggerFromContext` and `WithLogger` APIs.
- `.agent-specstar/features/logging/design.md` — log emission points and level policy.

## Allowed Write Paths

- `internal/application/deposit_service.go` (MODIFY)
- `internal/application/withdraw_service.go` (MODIFY)
- `internal/application/deposit_service_test.go` (MODIFY)
- `internal/application/withdraw_service_test.go` (MODIFY)

## Forbidden Paths

- `internal/adapters/`
- `internal/domain/`
- `internal/ports/`
- `internal/platform/`
- `cmd/`

## Known Abstraction Opportunities

- None. Logging calls are direct `logger.Error(...)` / `logger.Info(...)` calls inline with the error handling flow.

## Allowed Abstraction Scope

- None.

## Required Tests

**deposit_service_test.go:**
1. Existing tests continue to pass (no constructor change, but tests must provide logger via `platform.WithLogger` in context).
2. New test: processed command query infra failure (5xx) → verify ERROR log with `error`, `outcome=failed`.
3. New test: idempotency replay (existing command found) → verify INFO log with `order_id`, `outcome=replayed`.
4. New test: race condition replay (ErrDuplicate) → verify INFO log with `order_id`, `outcome=replayed`.
5. New test: unit-of-work infra failure (5xx) → verify ERROR log.
6. Verify no logging occurs for 4xx validation paths (client not found, asset not found).

**withdraw_service_test.go:**
1. Same pattern as deposit service.
2. Verify idempotency replay logging.
3. Verify race condition replay logging.
4. Verify terminal infra failure logging.
5. Verify no logging for ErrNotFound, ErrInsufficientPosition, ErrConcurrencyConflict paths.

**Test setup pattern:**
- Create test logger via `slog.New(slog.NewJSONHandler(&buf, nil))`.
- Store in context via `platform.WithLogger(ctx, testLogger)`.
- Pass enriched ctx to service mocks and Execute call.
- Parse buffer output to verify log entries.

## Coverage Requirement

100% on all changed lines in `deposit_service.go` and `withdraw_service.go`.

## Failure Model

- Logging calls are fire-and-forget. They do not affect service return values or error propagation.
- If `LoggerFromContext(ctx)` returns `slog.Default()` (no logger in context), log entries still emit to stdout using the global handler. Service behavior is unchanged.

## Allowed Fallbacks

- None. `LoggerFromContext` fallback to `slog.Default()` is an existing design decision from step-001.

## Acceptance Criteria

1. `go test ./internal/application/...` passes with 100% coverage on changed lines.
2. Service constructor signatures are unchanged.
3. `fmt.Errorf` calls are preserved unchanged.
4. Terminal 5xx failures produce ERROR log entries with `error` and `outcome=failed` fields.
5. Idempotency replays produce INFO log entries with `order_id` and `outcome=replayed` fields.
6. No log entries are produced for 4xx validation failures.
7. All existing tests in the repository continue to pass.

## Deferred Work

- None.

## Escalation Conditions

- If existing test mocks use `context.Background()` and adding `platform.WithLogger` causes unexpected behavior, escalate to determine test setup strategy.
- If the `platform` import from `application` causes import cycle concerns, escalate (it should not — `platform` does not import `application`).
