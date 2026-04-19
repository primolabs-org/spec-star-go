# Step 001: Surface rollback error in transaction.go

## Objective

Check the error returned by `tx.Rollback()` in `TransactionRunner.Do`. When both the `fn` error and the rollback error are non-nil, combine them with `errors.Join` so both are surfaced. When only `fn` fails and rollback succeeds, return the `fn` error unchanged.

## In Scope

- `internal/adapters/outbound/postgres/transaction.go` — fix `_ = tx.Rollback(ctx)`, add `"errors"` import.
- `internal/adapters/outbound/postgres/transaction_test.go` — add rollback-failure integration test.

## Out of Scope

- Changes to `deposit_service.go` (step 002).
- Changes to the `UnitOfWork` port interface.
- New logging or observability for rollback failures.

## Required Reads

- `internal/adapters/outbound/postgres/transaction.go`
- `internal/adapters/outbound/postgres/transaction_test.go`
- `internal/adapters/outbound/postgres/helpers.go` (for `txKey{}`)

## Allowed Write Paths

- `internal/adapters/outbound/postgres/transaction.go`
- `internal/adapters/outbound/postgres/transaction_test.go`

## Forbidden Paths

- `internal/application/`
- `internal/domain/`
- `internal/ports/`
- `internal/adapters/inbound/`

## Implementation Detail

Replace:
```go
if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
    _ = tx.Rollback(ctx)
    return err
}
```

With:
```go
if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
    if rbErr := tx.Rollback(ctx); rbErr != nil {
        return errors.Join(err, rbErr)
    }
    return err
}
```

Add `"errors"` to the import block.

## Known Abstraction Opportunities

None.

## Allowed Abstraction Scope

None.

## Required Tests

1. **`TestTransactionRunner_RollbackFailure_JoinsBothErrors`** (integration, `//go:build integration`): Cancel the parent context inside `fn` before returning an error. Assert:
   - The returned error is non-nil.
   - `errors.Is(err, fnError)` is true (the fn error is preserved).
   - `errors.Is(err, context.Canceled)` is true (the rollback error is surfaced).
2. **Existing tests pass unchanged**: `TestTransactionRunner_Commit`, `TestTransactionRunner_Rollback`, `TestTransactionRunner_NestedRepositoryCalls`.

## Coverage Requirement

100% on changed lines in `transaction.go`. The `if rbErr != nil` branch and `errors.Join` return are both exercised by the new test. The `return err` fallback (rollback succeeded) is exercised by the existing `TestTransactionRunner_Rollback`.

## Failure Model

- `fn` fails, rollback fails: return `errors.Join(fnErr, rbErr)`.
- `fn` fails, rollback succeeds: return `fnErr` unchanged.
- `fn` succeeds: proceed to commit (unchanged behavior).

## Allowed Fallbacks

None. Both errors must be surfaced.

## Acceptance Criteria

1. `_ = tx.Rollback(ctx)` is replaced with error-checked rollback using `errors.Join`.
2. The `"errors"` import is present in `transaction.go`.
3. The new integration test passes and validates both errors are preserved via `errors.Is`.
4. All existing integration tests pass unchanged.
5. `grep '_ =' internal/adapters/outbound/postgres/transaction.go` returns zero matches.

## Deferred Work

None.

## Escalation Conditions

- If pgx does not return an error for `tx.Rollback` on a cancelled context in integration tests, escalate: the test strategy must be revised (e.g., close pool inside `fn` instead).
