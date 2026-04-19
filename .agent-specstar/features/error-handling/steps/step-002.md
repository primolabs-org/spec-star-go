# Step 002: Propagate NewPosition and json.Marshal errors in deposit_service.go

## Objective

Check the errors returned by `domain.NewPosition()` and `json.Marshal()` in `DepositService.Execute`. Return `http.StatusInternalServerError` with wrapped context if either fails. Remove the stale justification comments above each call.

## In Scope

- `internal/application/deposit_service.go` — fix both `_ =` discards, remove stale comments.
- `internal/application/deposit_service_test.go` — verify existing tests cover changed assignment lines.

## Out of Scope

- Changes to `transaction.go` (covered by step 001).
- Changes to domain validation rules.
- Adding new test cases for unreachable error branches.

## Required Reads

- `internal/application/deposit_service.go`
- `internal/application/deposit_service_test.go`
- `internal/domain/position.go` (`NewPosition` signature and validation rules)

## Allowed Write Paths

- `internal/application/deposit_service.go`
- `internal/application/deposit_service_test.go`

## Forbidden Paths

- `internal/domain/`
- `internal/ports/`
- `internal/adapters/`
- `internal/platform/`

## Implementation Detail

Replace (around line 108-109):
```go
// Service validates strictly positive; NewPosition rejects only negative — cannot fail here.
position, _ := domain.NewPosition(clientID, assetID, amount, unitPrice, time.Now().UTC())
```

With:
```go
position, err := domain.NewPosition(clientID, assetID, amount, unitPrice, time.Now().UTC())
if err != nil {
    return nil, http.StatusInternalServerError, fmt.Errorf("new position: %w", err)
}
```

Replace (around line 112-113):
```go
// DepositResponse contains only string fields; json.Marshal cannot fail.
snapshotBytes, _ := json.Marshal(resp)
```

With:
```go
snapshotBytes, err := json.Marshal(resp)
if err != nil {
    return nil, http.StatusInternalServerError, fmt.Errorf("marshal response snapshot: %w", err)
}
```

## Known Abstraction Opportunities

None.

## Allowed Abstraction Scope

None.

## Required Tests

1. **Existing tests pass unchanged**: All tests in `deposit_service_test.go` must pass with no behavior change. The happy-path tests (`TestExecute_ValidDeposit_Returns201`, `TestExecute_ValidDeposit_ResponseFieldsMatchPosition`) cover the changed assignment lines.
2. **No new tests for unreachable branches**: The `NewPosition` error branch is unreachable because `validateDepositRequest` enforces strictly positive amounts while `NewPosition` only rejects negative. The `json.Marshal` error branch is unreachable because `DepositResponse` contains only string fields. These defensive checks exist per error-handling/fail-fast rules. The called functions are independently tested in `domain/position_test.go` and stdlib respectively.

## Coverage Requirement

The assignment lines (`position, err := ...` and `snapshotBytes, err := ...`) are covered by existing happy-path tests. The error branch bodies (`return nil, http.StatusInternalServerError, ...`) are unreachable from the public API and constitute a documented narrow coverage exception. This is accepted because:
- The errors are unreachable due to upstream validation in the same method.
- The called functions are independently tested in their own packages.
- The checks exist solely to prevent future regressions if upstream preconditions change.
- Introducing testability abstractions (e.g., injecting `NewPosition` as a dependency) would be over-engineering for defensive-only code.

## Failure Model

- If `domain.NewPosition()` fails: return `(nil, 500, "new position: <wrapped error>")`.
- If `json.Marshal()` fails: return `(nil, 500, "marshal response snapshot: <wrapped error>")`.

## Allowed Fallbacks

None.

## Acceptance Criteria

1. `position, _ := domain.NewPosition(...)` is replaced with `position, err := ...` followed by an error check returning 500.
2. `snapshotBytes, _ := json.Marshal(resp)` is replaced with `snapshotBytes, err := ...` followed by an error check returning 500.
3. Both stale justification comments above the calls are removed.
4. All existing unit tests pass unchanged with no behavior change.
5. `grep '_ =' internal/application/deposit_service.go` returns zero matches for discarded error returns.

## Deferred Work

None.

## Escalation Conditions

None expected. The changes are mechanical error propagation additions.
