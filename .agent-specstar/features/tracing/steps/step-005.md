# Step 005 — Application service instrumentation: child spans

## Metadata
- Feature: tracing
- Step: step-005
- Status: pending
- Depends On: step-001
- Last Updated: 2026-04-19

## Objective

Instrument `DepositService.Execute` and `WithdrawService.Execute` with child spans representing use-case execution. Each service method creates a span that captures the operation outcome including idempotent replay detection.

## In Scope

- Modify `deposit_service.go`:
  - Start a child span named `deposit.execute` using `otel.Tracer("application")`.
  - Defer `span.End()`.
  - Set `wallet.order_id` attribute.
  - Set `wallet.outcome` attribute: `success`, `failed`, or `replayed` based on the execution path.
  - On error: set span status Error with description and record the error.
  - On success: set span status OK.
  - On replay: set span status OK with `wallet.outcome=replayed`.
- Modify `withdraw_service.go`:
  - Same pattern with span name `withdraw.execute`.
- Update existing service tests to verify:
  - Child spans are created with expected names and attributes.
  - Outcome attributes are correct for success, failure, and replay paths.

## Out of Scope

- Handler root spans (step-004).
- Repository child spans (step-006).
- Metrics recording (owned by step-004 at handler level).
- Any changes to domain logic or service behavior.

## Required Reads

- `.agent-specstar/features/tracing/design.md` — FR-3.
- `internal/application/deposit_service.go` — current implementation.
- `internal/application/withdraw_service.go` — current implementation.
- `internal/application/deposit_service_test.go` — existing test patterns.
- `internal/application/withdraw_service_test.go` — existing test patterns.
- `.github/skills/go-lambda-observability-otel/SKILL.md` — application layer rules.

## Allowed Write Paths

- `internal/application/deposit_service.go` (MODIFY)
- `internal/application/deposit_service_test.go` (MODIFY)
- `internal/application/withdraw_service.go` (MODIFY)
- `internal/application/withdraw_service_test.go` (MODIFY)

## Forbidden Paths

- `internal/platform/**`
- `internal/adapters/**`
- `internal/domain/**`
- `internal/ports/**`
- `cmd/**`

## Known Abstraction Opportunities

- A package-level `var tracer = otel.Tracer("application")` to avoid repeated tracer acquisition.

## Allowed Abstraction Scope

- Package-level tracer variable in the `application` package. No new files.

## Required Tests

In `deposit_service_test.go`:
1. Successful deposit creates a span named `deposit.execute` with status OK and `wallet.outcome=success`.
2. Idempotent replay creates a span with `wallet.outcome=replayed`.
3. Validation failure creates a span with status Error and `wallet.outcome=failed`.
4. Infrastructure failure (e.g., repository error) creates a span with status Error and `wallet.outcome=failed`.
5. Span includes `wallet.order_id` attribute.

In `withdraw_service_test.go`:
1. Successful withdrawal creates a span named `withdraw.execute` with status OK and `wallet.outcome=success`.
2. Idempotent replay creates a span with `wallet.outcome=replayed`.
3. Failure creates a span with status Error and `wallet.outcome=failed`.
4. Span includes `wallet.order_id` attribute.

Test approach: Use a test `TracerProvider` with in-memory span exporter. The existing test doubles for repositories remain unchanged.

## Coverage Requirement

100% on all changed lines in `deposit_service.go` and `withdraw_service.go`.

## Failure Model

- Span creation from `otel.Tracer("application")` never fails (returns noop if provider not set).
- Service behavior (return values, error propagation) must not change.
- Tracing is additive; all existing tests must continue passing.

## Allowed Fallbacks

- none

## Acceptance Criteria

1. Both service files compile with no errors.
2. `go test ./internal/application/...` passes with 100% coverage on changed lines.
3. Span names: `deposit.execute`, `withdraw.execute`.
4. Span attributes include `wallet.order_id` and `wallet.outcome`.
5. Span status is set correctly for success, failure, and replay paths.
6. Existing service behavior is unchanged.

## Deferred Work

- none

## Escalation Conditions

- If the existing service interface or test setup makes it difficult to inject a test TracerProvider, escalate to determine whether dependency injection for tracer is needed vs global provider approach.
