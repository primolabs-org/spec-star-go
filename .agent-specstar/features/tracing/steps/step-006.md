# Step 006 — Repository and transaction instrumentation: database spans

## Metadata
- Feature: tracing
- Step: step-006
- Status: pending
- Depends On: step-001
- Last Updated: 2026-04-19

## Objective

Instrument the Postgres repository methods and `TransactionRunner.Do` with child spans representing database operations. Each significant repository call creates a span with database-oriented attributes.

## In Scope

- Modify repository files in `internal/adapters/outbound/postgres/`:
  - `processed_command_repository.go` — spans for `FindByTypeAndOrderID` and `Create`.
  - `client_repository.go` — span for `FindByID`.
  - `asset_repository.go` — span for `FindByID`.
  - `position_repository.go` — spans for `FindByClientAndInstrument`, `Create`, `Update`.
  - `transaction.go` — span for `Do` (wrapping the transaction boundary).
- Span naming convention: `db.<entity>.<operation>`:
  - `db.processed_command.find_by_type_and_order_id`
  - `db.processed_command.create`
  - `db.client.find_by_id`
  - `db.asset.find_by_id`
  - `db.position.find_by_client_and_instrument`
  - `db.position.create`
  - `db.position.update`
  - `db.transaction`
- Attributes on each span: `db.system=postgresql`, `db.operation.name` (e.g., `SELECT`, `INSERT`, `UPDATE`).
- On error: set span status Error with description and record the error.
- On success: set span status OK.
- Use `otel.Tracer("postgres")` for all repository/transaction spans.
- Update existing repository and transaction tests to verify span creation, naming, and attributes.
- Consider adding a small helpers or pattern to reduce span boilerplate if the pattern is highly repetitive.

## Out of Scope

- Handler root spans (step-004).
- Application service child spans (step-005).
- Trace-log correlation (step-002).
- Any changes to query logic, error handling, or repository behavior.
- pgx automatic tracing hook (deferred per design.md open question 4).

## Required Reads

- `.agent-specstar/features/tracing/design.md` — FR-4.
- `internal/adapters/outbound/postgres/position_repository.go` — current implementation.
- `internal/adapters/outbound/postgres/client_repository.go` — current implementation.
- `internal/adapters/outbound/postgres/asset_repository.go` — current implementation.
- `internal/adapters/outbound/postgres/processed_command_repository.go` — current implementation.
- `internal/adapters/outbound/postgres/transaction.go` — current implementation.
- `internal/adapters/outbound/postgres/helpers.go` — existing helper patterns.
- `internal/adapters/outbound/postgres/testhelper_test.go` — existing test infrastructure.
- `.github/skills/go-lambda-observability-otel/SKILL.md` — outbound adapter rules.

## Allowed Write Paths

- `internal/adapters/outbound/postgres/asset_repository.go` (MODIFY)
- `internal/adapters/outbound/postgres/asset_repository_test.go` (MODIFY)
- `internal/adapters/outbound/postgres/client_repository.go` (MODIFY)
- `internal/adapters/outbound/postgres/client_repository_test.go` (MODIFY)
- `internal/adapters/outbound/postgres/position_repository.go` (MODIFY)
- `internal/adapters/outbound/postgres/position_repository_test.go` (MODIFY)
- `internal/adapters/outbound/postgres/processed_command_repository.go` (MODIFY)
- `internal/adapters/outbound/postgres/processed_command_repository_test.go` (MODIFY)
- `internal/adapters/outbound/postgres/transaction.go` (MODIFY)
- `internal/adapters/outbound/postgres/transaction_test.go` (MODIFY)
- `internal/adapters/outbound/postgres/helpers.go` (MODIFY — if adding a span helper)

## Forbidden Paths

- `internal/platform/**`
- `internal/application/**`
- `internal/domain/**`
- `internal/ports/**`
- `internal/adapters/inbound/**`
- `cmd/**`

## Known Abstraction Opportunities

- A small span-start helper in the `postgres` package (e.g., `startSpan(ctx, name, dbOp string) (context.Context, trace.Span)`) to reduce repetitive tracer/attribute setup. Each repository call still owns its own span name.

## Allowed Abstraction Scope

- One unexported helper function in `helpers.go` or inline in each file. No new files.

## Required Tests

For each modified repository file, verify:
1. The instrumented method creates a span with the expected name.
2. The span includes `db.system=postgresql` and `db.operation.name` attributes.
3. On success: span status is OK.
4. On error: span status is Error with recorded error.

For `transaction.go`:
1. `Do` creates a span named `db.transaction`.
2. On successful commit: span status OK.
3. On rollback (fn returns error): span status Error.
4. On begin error: span status Error.

Test approach: Use test `TracerProvider` with in-memory span exporter. The existing repository test infrastructure (mock pool, fake queries) remains applicable.

## Coverage Requirement

100% on all changed lines across all modified files.

## Failure Model

- Span creation never fails (returns noop span if provider not configured).
- Repository and transaction behavior (return values, error handling, rollback logic) must not change.
- Tracing is additive; all existing tests must continue passing.

## Allowed Fallbacks

- none

## Acceptance Criteria

1. All modified files compile with no errors.
2. `go test ./internal/adapters/outbound/postgres/...` passes with 100% coverage on changed lines.
3. Span names follow `db.<entity>.<operation>` convention.
4. Span attributes include `db.system` and `db.operation.name`.
5. Error spans are set correctly on database failures.
6. Existing repository and transaction behavior is unchanged.

## Deferred Work

- pgx built-in tracer hook evaluation (design.md open question 4) is deferred to a future feature.

## Escalation Conditions

- If the existing test infrastructure (mock pool, fake pgx responses) makes it difficult to verify spans alongside database behavior, propose an approach for span assertions that works with the existing test doubles.
