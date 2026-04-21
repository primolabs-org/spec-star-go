# Step 006 Fix 001 Review - tracing

## Metadata
- Feature: tracing
- Step: step-006-fix-001
- Base step: step-006
- Prior review: step-006-review
- Reviewer mode: SpecStar Reviewer
- Date: 2026-04-21

## Verdict
APPROVED

## Findings

### 1) Previous rejection finding #1 is fully resolved (disallowed test helper path)
- No step-006 tracing helper additions remain in `internal/adapters/outbound/postgres/testhelper_test.go`.
- Span assertion helpers now live in an allowed file: `internal/adapters/outbound/postgres/transaction_test.go` (e.g., `setupPostgresTestTracer` at line 155, `requireDBSpanSuccess` at line 173, `requireDBSpanError` at line 185).

### 2) Previous rejection finding #2 is fully resolved (abstraction limit)
- `internal/adapters/outbound/postgres/helpers.go` now contains one unexported tracing helper only: `startDBSpan` (line 46).
- The previously rejected second helper (`setSpanStatus`) is no longer present.

### 3) Required step-006 span behavior remains correct
- Required span names are present in production methods:
  - `db.asset.find_by_id` in `internal/adapters/outbound/postgres/asset_repository.go:32`
  - `db.client.find_by_id` in `internal/adapters/outbound/postgres/client_repository.go:29`
  - `db.position.find_by_client_and_instrument` in `internal/adapters/outbound/postgres/position_repository.go:63`
  - `db.position.create` in `internal/adapters/outbound/postgres/position_repository.go:96`
  - `db.position.update` in `internal/adapters/outbound/postgres/position_repository.go:119`
  - `db.processed_command.find_by_type_and_order_id` in `internal/adapters/outbound/postgres/processed_command_repository.go:32`
  - `db.processed_command.create` in `internal/adapters/outbound/postgres/processed_command_repository.go:69`
  - `db.transaction` in `internal/adapters/outbound/postgres/transaction.go:30`
- Span attributes remain centralized in helper:
  - `db.system=postgresql` and `db.operation.name` are set in `internal/adapters/outbound/postgres/helpers.go:46`.
- Error/success behavior remains compliant:
  - Error paths set status `Error` and call `RecordError` in all required methods.
  - Success paths set status `Ok` in all required methods.
- Tests assert required spans and operations in allowed test files:
  - `asset_repository_test.go:77,111`
  - `client_repository_test.go:42,59`
  - `position_repository_test.go:77,159,176,224,269,320`
  - `processed_command_repository_test.go:64,65,82,113`
  - `transaction_test.go:47,80,99`

### 4) Required validation command evidence is acceptable
- Required command executed:
  - `go test -tags integration ./internal/adapters/outbound/postgres/... -coverprofile=/tmp/step006_fix_cover.out`
- Result: PASS when `TEST_DATABASE_URL` is provided.
- Package coverage result: `96.8%`.
- Function coverage for changed instrumented methods remains `100%` (including `FindByID`, `FindByTypeAndOrderID`, `Create`, `Update`, `FindByClientAndInstrument`, `Do`, and `startDBSpan`).

### 5) Allowed path compliance
- All non-workflow implementation edits are within the allowed write paths listed by step-006-fix-001.
- One workflow metadata file is also modified: `.agent-specstar/features/tracing/feature-state.json`. This is not application source/test/config behavior for step-006 and does not affect product-scope compliance.

## Conclusion
The two prior rejection findings are fully fixed, step-006 tracing behavior remains correct, required validation evidence is acceptable, and implementation scope is compliant for product files. Step `step-006-fix-001` is approved.
