# Step 002 Fix 001 Review - tracing

## Metadata
- Feature: tracing
- Step: step-002-fix-001
- Reviewer mode: SpecStar Reviewer
- Date: 2026-04-21
- Trigger review: `.agent-specstar/features/tracing/reviews/step-007-review.md`

## Verdict
APPROVED

## Review Scope
- Step contract: `.agent-specstar/features/tracing/steps/step-002-fix-001.md`
- Design context: `.agent-specstar/features/tracing/design.md` (FR-6, Success Criteria #4/#5)
- Implementation reviewed in working tree:
  - `internal/adapters/inbound/httphandler/deposit_handler.go`
  - `internal/adapters/inbound/httphandler/withdraw_handler.go`
  - `internal/adapters/inbound/httphandler/deposit_handler_test.go`
  - `internal/adapters/inbound/httphandler/withdraw_handler_test.go`
  - `internal/adapters/outbound/postgres/asset_repository.go`
  - `internal/adapters/outbound/postgres/client_repository.go`
  - `internal/adapters/outbound/postgres/position_repository.go`
  - `internal/adapters/outbound/postgres/processed_command_repository.go`
  - `internal/adapters/outbound/postgres/transaction.go`
  - `internal/adapters/outbound/postgres/logging_test.go`

## Findings

### 1) Runtime trace-log correlation regression is fixed in active span error paths
Inbound terminal logging now uses context-aware calls so active span context reaches the logger handler:
- `internal/adapters/inbound/httphandler/deposit_handler.go:77`
- `internal/adapters/inbound/httphandler/deposit_handler.go:84`
- `internal/adapters/inbound/httphandler/deposit_handler.go:87`
- `internal/adapters/inbound/httphandler/withdraw_handler.go:59`

Outbound Postgres error logs in tracing-instrumented runtime paths now use `ErrorContext` with request context:
- `internal/adapters/outbound/postgres/processed_command_repository.go:57`
- `internal/adapters/outbound/postgres/transaction.go:35`
- `internal/adapters/outbound/postgres/transaction.go:55`
- plus the same correction pattern in:
  - `internal/adapters/outbound/postgres/asset_repository.go`
  - `internal/adapters/outbound/postgres/client_repository.go`
  - `internal/adapters/outbound/postgres/position_repository.go`

### 2) Regression tests were added for inbound/outbound correlation behavior
Inbound correlation assertions with active spans:
- `internal/adapters/inbound/httphandler/deposit_handler_test.go:458`
- `internal/adapters/inbound/httphandler/withdraw_handler_test.go:371`

Outbound correlation assertions with active spans:
- `internal/adapters/outbound/postgres/logging_test.go:564`
- `internal/adapters/outbound/postgres/logging_test.go:669`
- `internal/adapters/outbound/postgres/logging_test.go:714`

### 3) Required validations are satisfied
Executed required commands from the fix-step:
- `go test ./internal/adapters/inbound/httphandler/...` -> PASS
- `go test ./internal/adapters/outbound/postgres/... -run TestLogging` -> PASS

Manual runtime correlation checks (re-run):
- `docker compose logs wallet-api` contains structured JSON log entries with non-empty `trace_id` and `span_id` in active-span terminal error logs.
- Loki query using the checklist selector `{container=~".*wallet-api.*"}` returns `wallet-api` structured log entries, including a `request failed` record containing `trace_id`/`span_id`.

### 4) Scope compliance
Product-code and test edits for this fix are within the fix-step allowed runtime/test paths listed in the step contract.

Note: the working tree still contains prior step-007 documentation/workflow changes (`README.md`, `.agent-specstar/features/tracing/feature-state.json`), which are outside this fix-step product scope but are pre-existing workflow carryover from the rejected step-007 context.

## Acceptance Criteria Check
1. Runtime error logs in active trace contexts include non-empty `trace_id` and `span_id`: PASS
2. Existing log field model remains intact (`service`, `trigger`, `operation`, etc.): PASS
3. Regression tests for inbound and outbound correlation pass: PASS
4. Step-007 log-correlation validation items pass without README workaround: PASS
5. No fix-step product-code edits outside allowed paths: PASS

## Outcome
Step `step-002-fix-001` is approved and ready for workflow progression.
