# Step 006 Fix 001 - Scope compliance and abstraction limit

## Metadata
- Feature: tracing
- Step: step-006-fix-001
- Status: pending
- Depends On: step-006
- Trigger: review rejection in `reviews/step-006-review.md`

## Objective
Bring step-006 into strict contract compliance without changing repository or transaction behavior.

## In Scope
- Remove step-006 test instrumentation helpers from disallowed path:
  - `internal/adapters/outbound/postgres/testhelper_test.go`
- Keep span assertions in allowed test files only:
  - `internal/adapters/outbound/postgres/asset_repository_test.go`
  - `internal/adapters/outbound/postgres/client_repository_test.go`
  - `internal/adapters/outbound/postgres/position_repository_test.go`
  - `internal/adapters/outbound/postgres/processed_command_repository_test.go`
  - `internal/adapters/outbound/postgres/transaction_test.go`
- Refactor tracing helper usage to satisfy abstraction limit in `helpers.go`:
  - keep only one unexported helper abstraction in `internal/adapters/outbound/postgres/helpers.go`, or
  - inline the second helper logic in call sites.
- Preserve all required span names, attributes, and status/error-recording behavior from step-006.

## Out of Scope
- Any query logic changes.
- Any domain/application/inbound/cmd changes.
- Any new files.

## Allowed Write Paths
- `internal/adapters/outbound/postgres/asset_repository.go`
- `internal/adapters/outbound/postgres/asset_repository_test.go`
- `internal/adapters/outbound/postgres/client_repository.go`
- `internal/adapters/outbound/postgres/client_repository_test.go`
- `internal/adapters/outbound/postgres/position_repository.go`
- `internal/adapters/outbound/postgres/position_repository_test.go`
- `internal/adapters/outbound/postgres/processed_command_repository.go`
- `internal/adapters/outbound/postgres/processed_command_repository_test.go`
- `internal/adapters/outbound/postgres/transaction.go`
- `internal/adapters/outbound/postgres/transaction_test.go`
- `internal/adapters/outbound/postgres/helpers.go`

## Forbidden Paths
- `internal/platform/**`
- `internal/application/**`
- `internal/domain/**`
- `internal/ports/**`
- `internal/adapters/inbound/**`
- `cmd/**`
- `internal/adapters/outbound/postgres/testhelper_test.go`

## Required Validation
1. Run:
   - `go test -tags integration ./internal/adapters/outbound/postgres/... -coverprofile=/tmp/step006_fix_cover.out`
2. Confirm:
   - tests pass
   - required span assertions remain present
   - changed production functions remain fully covered

## Acceptance Criteria
1. No step-006 helper additions remain in `internal/adapters/outbound/postgres/testhelper_test.go`.
2. Abstraction scope in `internal/adapters/outbound/postgres/helpers.go` complies with one-helper limit.
3. Required step-006 span names/attributes/status behavior remains correct.
4. Integration tests for `internal/adapters/outbound/postgres` pass.
5. No edits outside allowed write paths.