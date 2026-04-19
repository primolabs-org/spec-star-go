# Step 002 Fix 001 - Cover json.Marshal error branch

## Metadata
- Feature: withdraw
- Step: step-002-fix-001
- Status: pending
- Depends On: step-002
- Last Updated: 2026-04-19

## Problem

`withdraw_service.go` line 140-142 (the `json.Marshal(resp)` error branch inside `UnitOfWork.Do`) is uncovered. Coverage is 98.5%, requirement is 100%.

`WithdrawResponse` contains only `string` fields, so `json.Marshal` cannot fail at runtime. The error check is correct per error-handling instructions (propagate even when unreachable), but it needs a test seam to be exercisable.

## Required Change

### In `withdraw_service.go`

1. Add a package-level variable following the existing `deposit_handler.go` pattern:

```go
var withdrawMarshalJSON = json.Marshal
```

2. Replace the direct `json.Marshal(resp)` call on line 139 with:

```go
snapshotBytes, err := withdrawMarshalJSON(resp)
```

No other changes.

### In `withdraw_service_test.go`

Add one test:

```go
func TestWithdrawExecute_MarshalSnapshotError_Returns500(t *testing.T) {
    // Override withdrawMarshalJSON to return an error
    original := withdrawMarshalJSON
    withdrawMarshalJSON = func(v any) ([]byte, error) {
        return nil, errors.New("marshal failed")
    }
    t.Cleanup(func() { withdrawMarshalJSON = original })

    // Set up mocks with a valid single-lot withdrawal scenario
    // Execute and verify status 500 and non-nil error
}
```

## Scope

- `internal/application/withdraw_service.go` — one variable addition, one call-site change
- `internal/application/withdraw_service_test.go` — one new test

## Acceptance Criteria

1. `withdraw_service.go` reaches 100% statement coverage.
2. The new test verifies the marshal error returns status 500 with a non-nil error.
3. All existing tests continue to pass.
4. No other files modified.
