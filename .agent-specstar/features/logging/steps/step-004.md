# Step 004 — Outbound adapters: dependency failure logging

## Objective

Add structured logging to outbound PostgreSQL adapters for infrastructure-level database failures using the correlated logger from context. Add unit tests with mock `DBTX` to verify logging behavior on error paths.

## In Scope

- **`ClientRepository`**: log unexpected query errors in `FindByID` and `Create` at ERROR level with entity identifier and error detail. Do NOT log `ErrNoRows` (mapped to `domain.ErrNotFound`).
- **`AssetRepository`**: log unexpected query errors in `FindByID`, `FindByInstrumentID`, `scanAsset`, and `Create` at ERROR level. Do NOT log `ErrNoRows`.
- **`PositionRepository`**: log unexpected query/exec errors in `FindByID`, `FindByClientAndAsset`, `FindByClientAndInstrument`, `Create`, `Update` at ERROR level. Do NOT log `ErrNoRows`. Do NOT log `ErrConcurrencyConflict` (business outcome from `Update` with 0 rows affected).
- **`ProcessedCommandRepository`**: log unexpected query/exec errors in `FindByTypeAndOrderID` and `Create` at ERROR level. Do NOT log `ErrNoRows`. Do NOT log unique violation mapped to `ErrDuplicate`.
- **`TransactionRunner`**: log `Begin` and `Commit` failures at ERROR level in `Do` method. Do NOT log `Rollback` failure (it is already joined with the original error via `errors.Join`).
- All logging uses `platform.LoggerFromContext(ctx)` to retrieve the correlated logger.
- Create unit tests using mock `DBTX` injected via `txKey{}` context key to trigger infrastructure errors and verify log output.

## Out of Scope

- Changing repository constructor signatures. Logger comes from context.
- Logging `ErrNoRows` / `domain.ErrNotFound` paths.
- Logging `domain.ErrDuplicate` paths.
- Logging `domain.ErrConcurrencyConflict` paths (business outcomes).
- Logging successful query/exec results.
- Modifying integration tests behind `//go:build integration`.
- Changing `fmt.Errorf` calls — error wrapping is preserved.
- Modifying `helpers.go` (the `DBTX` interface and `executorFromContext` are unchanged).

## Required Reads

- `internal/adapters/outbound/postgres/client_repository.go`
- `internal/adapters/outbound/postgres/asset_repository.go`
- `internal/adapters/outbound/postgres/position_repository.go`
- `internal/adapters/outbound/postgres/processed_command_repository.go`
- `internal/adapters/outbound/postgres/transaction.go`
- `internal/adapters/outbound/postgres/helpers.go` — understand `DBTX` interface, `txKey{}`, `executorFromContext`.
- `internal/platform/logger.go` (step-001) — for `LoggerFromContext` API.
- `.agent-specstar/features/logging/design.md` — log emission points for outbound adapters.

## Allowed Write Paths

- `internal/adapters/outbound/postgres/client_repository.go` (MODIFY)
- `internal/adapters/outbound/postgres/asset_repository.go` (MODIFY)
- `internal/adapters/outbound/postgres/position_repository.go` (MODIFY)
- `internal/adapters/outbound/postgres/processed_command_repository.go` (MODIFY)
- `internal/adapters/outbound/postgres/transaction.go` (MODIFY)
- `internal/adapters/outbound/postgres/logging_test.go` (CREATE — unit tests for logging with mock DBTX)

## Forbidden Paths

- `internal/adapters/outbound/postgres/helpers.go`
- `internal/adapters/outbound/postgres/testhelper_test.go`
- `internal/adapters/outbound/postgres/*_repository_test.go` (existing integration tests)
- `internal/adapters/outbound/postgres/transaction_test.go` (existing integration test)
- `internal/adapters/inbound/`
- `internal/application/`
- `internal/domain/`
- `internal/ports/`
- `internal/platform/`
- `cmd/`

## Known Abstraction Opportunities

- A shared test mock for `DBTX` could reduce boilerplate across repo logging tests. However, prefer inline mocks unless duplication becomes excessive within this single test file.

## Allowed Abstraction Scope

- A `mockDBTX` test type and optionally a `mockRow` test type (implementing `pgx.Row`) in the test file for reuse across test cases. These are test-only and private to the test file.

## Required Tests

All in `internal/adapters/outbound/postgres/logging_test.go` (no `//go:build integration` tag — these are fast unit tests):

**ClientRepository:**
1. `FindByID` with mock DBTX returning unexpected error → verify ERROR log with `client_id` and `error` fields.
2. `FindByID` with `pgx.ErrNoRows` → verify NO log entry.
3. `Create` with mock DBTX returning exec error → verify ERROR log.

**AssetRepository:**
1. `FindByID` with unexpected scan error → verify ERROR log.
2. `FindByID` with `pgx.ErrNoRows` → verify NO log entry.

**PositionRepository:**
1. `FindByID` with unexpected error → verify ERROR log.
2. `Create` with exec error → verify ERROR log.
3. `Update` with exec error → verify ERROR log.
4. `Update` with 0 rows affected (concurrency conflict) → verify NO log entry.

**ProcessedCommandRepository:**
1. `FindByTypeAndOrderID` with unexpected error → verify ERROR log.
2. `FindByTypeAndOrderID` with `pgx.ErrNoRows` → verify NO log entry.
3. `Create` with unique violation error → verify NO log entry.
4. `Create` with unexpected error → verify ERROR log.

**TransactionRunner:**
1. `Do` with `Begin` failure → verify ERROR log.
2. `Do` with `Commit` failure → verify ERROR log.

**Test setup pattern:**
- Create mock `DBTX` struct implementing the `DBTX` interface from `helpers.go`.
- Create mock `pgx.Row` for `QueryRow` return values where needed.
- Inject mock via `context.WithValue(ctx, txKey{}, mockDB)` so `executorFromContext` returns the mock.
- Use `bytes.Buffer`-backed `slog.JSONHandler` with `platform.WithLogger(ctx, testLogger)` to capture and verify log output.

## Coverage Requirement

100% on all changed lines in:
- `client_repository.go`
- `asset_repository.go`
- `position_repository.go`
- `processed_command_repository.go`
- `transaction.go`

## Failure Model

- Logging calls are fire-and-forget. They do not change error propagation or return values.
- Mock DBTX tests are fast in-memory tests with no I/O dependencies.

## Allowed Fallbacks

- None.

## Acceptance Criteria

1. `go test ./internal/adapters/outbound/postgres/...` passes (both new unit tests and existing integration tests if `TEST_DATABASE_URL` is set).
2. `go test ./internal/adapters/outbound/postgres/... -run TestLogging` runs the new unit tests without requiring a database.
3. 100% coverage on changed lines in production code.
4. Repository constructor signatures are unchanged.
5. `fmt.Errorf` calls are preserved unchanged.
6. Infrastructure errors produce ERROR log entries with entity identifier and error detail.
7. Business outcome errors (`ErrNotFound`, `ErrDuplicate`, `ErrConcurrencyConflict`) produce NO log entries.
8. All existing tests in the repository continue to pass.

## Deferred Work

- None.

## Escalation Conditions

- If mocking `pgx.Row` (for `QueryRow` return) is excessively complex due to pgx internals, consider alternative test strategies (e.g., wrapping the scan call in a testable helper) and escalate if needed.
- If `executorFromContext` returns the pool (not mock) because the context `txKey` value type doesn't match `pgx.Tx`, escalate to determine mock injection strategy. The mock must implement `DBTX`, not `pgx.Tx`, so injection approach may need adjustment.
