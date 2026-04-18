# Step 007 — Outbound PostgreSQL adapters

## Goal

Implement pgx-based repository adapters and the transaction runner that fulfill the port interfaces defined in step 004.

## Why

This step completes the persistence layer for the domain foundation. Adapters translate between domain entities and PostgreSQL rows, handle error mapping from pgx errors to domain error types, implement optimistic concurrency conflict detection, and provide the transaction boundary used by future application use cases.

## Depends on

- Step 003 (domain entities for construction and reconstitution).
- Step 004 (port interfaces that adapters implement).
- Step 005 (SQL schema that adapters query against).
- Step 006 (pgxpool for connection management).

## Required Reads

- `design.md` — "Repository Contracts" for method specifications, "Optimistic Concurrency" for the update-with-version-check pattern, "Idempotency" for unique constraint handling, "Persistence Schema" for column names and types.
- go-lambda-error-handling skill — adapter error translation rules, preserve cause with `%w`, keep domain errors transport-agnostic.
- go-aws-lambda-microservice-hexagonal skill — outbound adapter responsibilities, test placement rules.
- specstar-clean-code skill — cognitive complexity, naming, function design.

## In Scope

### Database executor abstraction (package-internal)

- A small `DBTX` interface within the adapter package that both `*pgxpool.Pool` and `pgx.Tx` satisfy (Exec, Query, QueryRow). This is a private implementation detail — it does not leak into ports.
- A context-based transaction extraction helper: when a `UnitOfWork` transaction is active, the adapter uses the `pgx.Tx` from context; otherwise, it falls back to the pool.

### ClientRepository (`internal/adapters/outbound/postgres/client_repository.go`)

- Implements `ports.ClientRepository`.
- `FindByID`: SELECT by `client_id`, reconstruct domain Client. Returns `domain.ErrNotFound` (wrapped) when `pgx.ErrNoRows`.
- `Create`: INSERT Client fields.

### AssetRepository (`internal/adapters/outbound/postgres/asset_repository.go`)

- Implements `ports.AssetRepository`.
- `FindByID`: SELECT by `asset_id`, reconstruct domain Asset. Returns `domain.ErrNotFound` when no rows.
- `FindByInstrumentID`: SELECT by `instrument_id`, reconstruct domain Asset. Returns `domain.ErrNotFound` when no rows.
- `Create`: INSERT Asset fields.

### PositionRepository (`internal/adapters/outbound/postgres/position_repository.go`)

- Implements `ports.PositionRepository`.
- `FindByID`: SELECT by `position_id`, reconstruct domain Position (including `row_version`). Returns `domain.ErrNotFound` when no rows.
- `FindByClientAndAsset`: SELECT by `(client_id, asset_id)`, return slice of domain Positions.
- `FindByClientAndInstrument`: SELECT with JOIN to `assets` on `(client_id, instrument_id)`, ordered by `purchased_at ASC`, return slice of domain Positions.
- `Create`: INSERT Position fields.
- `Update`: UPDATE with `WHERE position_id = $1 AND row_version = $2`. If zero rows affected, return `domain.ErrConcurrencyConflict` (wrapped). The adapter does NOT increment `row_version` — the domain entity handles that; the adapter writes the entity's current state.

### ProcessedCommandRepository (`internal/adapters/outbound/postgres/processed_command_repository.go`)

- Implements `ports.ProcessedCommandRepository`.
- `FindByTypeAndOrderID`: SELECT by `(command_type, order_id)`, reconstruct domain ProcessedCommand. Returns `domain.ErrNotFound` when no rows.
- `Create`: INSERT ProcessedCommand fields. Detects unique constraint violation on `(command_type, order_id)` and returns `domain.ErrDuplicate` (wrapped).

### Transaction runner (`internal/adapters/outbound/postgres/transaction.go`)

- Implements `ports.UnitOfWork`.
- Accepts `*pgxpool.Pool` in constructor.
- `Do(ctx, fn)`: begins a `pgx.Tx`, stores it in context, calls `fn`, commits on success, rolls back on error. Returns the `fn` error (if any) or a commit/begin error.
- Context key is unexported to prevent misuse from outside the adapter package.

### Error translation

- `pgx.ErrNoRows` → `domain.ErrNotFound` (wrapped with operation context).
- Zero-rows-affected on Position update → `domain.ErrConcurrencyConflict` (wrapped with position ID context).
- PostgreSQL unique constraint violation (pgx error code `23505`) on ProcessedCommand insert → `domain.ErrDuplicate` (wrapped with key context).
- Other pgx/database errors are wrapped and propagated without translation — they represent unexpected infrastructure failures.

### Decimal scanning

- `shopspring/decimal.Decimal` values are scanned from PostgreSQL `NUMERIC` columns via string-based scanning (decimal's `Scan()` method or explicit string conversion).
- Ensure no precision loss during scan/write.

## Out of Scope

- Inbound adapters (HTTP handlers, SQS consumers).
- Application use cases / command handlers.
- Deposit or withdraw logic.
- Read models or projections.
- Connection pool creation (step 006).
- Logger injection or structured logging in adapters (deferred to logging feature).

## Files to Create

- `internal/adapters/outbound/postgres/client_repository.go`
- `internal/adapters/outbound/postgres/asset_repository.go`
- `internal/adapters/outbound/postgres/position_repository.go`
- `internal/adapters/outbound/postgres/processed_command_repository.go`
- `internal/adapters/outbound/postgres/transaction.go`
- `internal/adapters/outbound/postgres/helpers.go` (DBTX interface, context tx extraction — only if needed to avoid repetition)
- `internal/adapters/outbound/postgres/client_repository_test.go`
- `internal/adapters/outbound/postgres/asset_repository_test.go`
- `internal/adapters/outbound/postgres/position_repository_test.go`
- `internal/adapters/outbound/postgres/processed_command_repository_test.go`
- `internal/adapters/outbound/postgres/transaction_test.go`

## Forbidden Paths

- `internal/domain/` — do not modify domain entities.
- `internal/ports/` — do not modify port interfaces.
- `internal/platform/` — do not modify platform configuration.
- `internal/application/`

## Known Abstraction Opportunities

- **DBTX interface**: abstract the common query surface of `*pgxpool.Pool` and `pgx.Tx`. Only create this if multiple repositories need the same abstraction; otherwise inline.
- **Context tx extraction**: a shared helper to extract `pgx.Tx` from context, shared across all repository implementations.
- **Row scanning**: if scanning patterns are highly repetitive, a small scan helper per entity may reduce duplication. Only extract after implementing all four repositories.

## Allowed Abstraction Scope

- Abstractions must stay within the `internal/adapters/outbound/postgres/` package.
- Must not leak into ports, domain, or platform packages.

## Required Tests

All adapter tests are **integration tests** that require a running PostgreSQL database. They must be guarded with a build tag: `//go:build integration`.

### `internal/adapters/outbound/postgres/client_repository_test.go`

- Create and find by ID: round-trip preserves all fields.
- Find non-existent ID: returns error wrapping `domain.ErrNotFound`.
- Create duplicate (same `client_id`): returns error (PK violation).

### `internal/adapters/outbound/postgres/asset_repository_test.go`

- Create and find by ID: round-trip preserves all fields including optional fields.
- Find by instrument ID: returns correct asset.
- Find non-existent ID: returns error wrapping `domain.ErrNotFound`.
- Find non-existent instrument ID: returns error wrapping `domain.ErrNotFound`.

### `internal/adapters/outbound/postgres/position_repository_test.go`

- Create and find by ID: round-trip preserves all fields including decimal precision.
- Find by client and asset: returns correct positions.
- Find by client and instrument: returns positions ordered by `purchased_at` ascending (requires asset with matching `instrument_id` to exist).
- Find by client and instrument with no matches: returns empty slice.
- Update with correct `row_version`: succeeds, `row_version` incremented in database.
- Update with stale `row_version`: returns error wrapping `domain.ErrConcurrencyConflict`.
- Find non-existent ID: returns error wrapping `domain.ErrNotFound`.
- Decimal precision: round-trip `amount × unit_price = total_value` holds exactly for representative values.

### `internal/adapters/outbound/postgres/processed_command_repository_test.go`

- Create and find by type and order ID: round-trip preserves all fields including `response_snapshot` JSONB.
- Find non-existent type/order: returns error wrapping `domain.ErrNotFound`.
- Create duplicate `(command_type, order_id)`: returns error wrapping `domain.ErrDuplicate`.

### `internal/adapters/outbound/postgres/transaction_test.go`

- Successful transaction: changes committed, visible after `Do` returns.
- Failed transaction (fn returns error): changes rolled back, not visible.
- Nested repository calls within `Do`: all operate on the same transaction.

### Test infrastructure

- Tests use a shared test helper that creates a `pgxpool.Pool` connected to a test PostgreSQL instance.
- Each test runs within a transaction that is rolled back after the test (or uses a dedicated schema/database) to ensure test isolation.
- The test database must have the schema from `migrations/001_initial_schema.sql` applied.

## Coverage Requirement

100% on all new lines. Integration tests count toward coverage when run with the integration build tag.

## Failure Model

- `ErrNotFound` on missing entities.
- `ErrConcurrencyConflict` on stale `row_version`.
- `ErrDuplicate` on unique constraint violation.
- Unexpected database errors propagated with context wrapping — no silent fallbacks.

## Allowed Fallbacks

None. All database failures are surfaced explicitly.

## Acceptance Criteria

- `go build ./...` succeeds.
- `go test ./internal/adapters/outbound/postgres/...` passes (with integration build tag and a running PostgreSQL instance).
- Compile-time interface satisfaction checks confirm all adapters implement their respective port interfaces.
- All error paths return the correct domain error type (verifiable via `errors.Is`).
- Optimistic concurrency: stale `row_version` produces `ErrConcurrencyConflict`.
- Idempotency: duplicate `(command_type, order_id)` produces `ErrDuplicate`.
- Decimal precision: no floating-point drift in round-trip persistence.
- Transaction runner: commits on success, rolls back on error.
- No domain, port, or platform package modifications.

## Deferred Work

- Structured logging in adapters is deferred to the logging bootstrap feature.
- Metrics/tracing instrumentation on database calls is deferred to the observability feature.
- Connection retry or circuit breaker logic is not in scope.

## Escalation Conditions

- If `shopspring/decimal.Decimal` does not scan cleanly from PostgreSQL `NUMERIC` via pgx, escalate to evaluate scanning strategy.
- If `pgx` does not expose a usable error code for unique constraint violations (`23505`), escalate to evaluate detection strategy.
- If test database infrastructure (Docker, testcontainers, or local PostgreSQL) is unavailable, escalate to determine integration test strategy.
