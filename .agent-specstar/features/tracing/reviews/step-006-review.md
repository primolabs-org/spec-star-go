# Step 006 Review - tracing

## Verdict
REJECTED

## Review Scope
- Step contract: `.agent-specstar/features/tracing/steps/step-006.md`
- Design context: `.agent-specstar/features/tracing/design.md` (FR-4)
- Implementation under review: current unstaged diff for Postgres adapter source/tests

## Findings

### 1) High - Edit outside allowed write paths
- The step allows writes only to the paths listed in `.agent-specstar/features/tracing/steps/step-006.md` lines 60-70.
- `internal/adapters/outbound/postgres/testhelper_test.go` is not in that list.
- The implementation adds substantial new test instrumentation helpers in `internal/adapters/outbound/postgres/testhelper_test.go` lines 89-178.
- Per reviewer policy, edits outside allowed write paths require rejection unless explicitly authorized.

### 2) Medium - Allowed abstraction scope exceeded
- The step constrains abstraction to one unexported helper in `helpers.go` (step file line 87).
- Implementation adds two unexported tracing helpers in `internal/adapters/outbound/postgres/helpers.go`:
  - `startDBSpan` at lines 47-56
  - `setSpanStatus` at lines 58-65
- This exceeds the explicit abstraction limit in the step contract.

## Acceptance Criteria Check (Evidence gathered)
Despite rejection, functional requirements are largely implemented correctly:

- Required span names are present in source:
  - `internal/adapters/outbound/postgres/processed_command_repository.go:31,66`
  - `internal/adapters/outbound/postgres/client_repository.go:28`
  - `internal/adapters/outbound/postgres/asset_repository.go:31`
  - `internal/adapters/outbound/postgres/position_repository.go:62,89,111`
  - `internal/adapters/outbound/postgres/transaction.go:29`
- Required attributes and tracer ownership are present in `internal/adapters/outbound/postgres/helpers.go:47-54` (`otel.Tracer("postgres")`, `db.system`, `db.operation.name`).
- Error/success span status handling is implemented via `setSpanStatus` in `internal/adapters/outbound/postgres/helpers.go:58-65` and used across repository/transaction methods.
- Tests assert span success/error cases with expected names and operations:
  - `internal/adapters/outbound/postgres/processed_command_repository_test.go:64-65,82,113`
  - `internal/adapters/outbound/postgres/client_repository_test.go:42,59`
  - `internal/adapters/outbound/postgres/asset_repository_test.go:77,111`
  - `internal/adapters/outbound/postgres/position_repository_test.go:77,159,176,224,269,320`
  - `internal/adapters/outbound/postgres/transaction_test.go:43,76,95`

## Test and Coverage Verification
Executed:
- `go test -tags integration ./internal/adapters/outbound/postgres/... -coverprofile=/tmp/step006_postgres_cover.out`

Result:
- PASS
- Package coverage: 96.7%
- Changed instrumented production functions in this step report 100% function coverage (`FindByID`, `FindByTypeAndOrderID`, `Create`, `Update`, `Do`, `startDBSpan`, `setSpanStatus`).

Environment note:
- Integration tests required `TEST_DATABASE_URL`; validated using local compose Postgres and `.env` values.

## Conclusion
Implementation behavior and tests are strong, but step-006 must be rejected due to contract non-compliance on allowed write paths (and abstraction scope). A focused fix-step is required.