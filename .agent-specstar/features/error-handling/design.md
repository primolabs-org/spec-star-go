# Error Handling Violations Fix

## Problem Statement

The deposit service (`deposit_service.go`) and transaction runner (`transaction.go`) contain three instances where error returns are silently discarded using `_ =` or blank identifiers, with stale comments justifying the discards. This violates the repository's mandatory error-handling and fail-fast rules regardless of whether the errors are currently reachable.

## Goal

Eliminate all discarded-error violations in the codebase so that every error return is checked and propagated, without changing existing business behavior or domain invariants.

## Functional Requirements

### FR-1: Propagate `domain.NewPosition()` error in `deposit_service.go`

**Location**: `internal/application/deposit_service.go`, `Execute` method, ~line 109.

**Current code**:
```go
// Service validates strictly positive; NewPosition rejects only negative — cannot fail here.
position, _ := domain.NewPosition(clientID, assetID, amount, unitPrice, time.Now().UTC())
```

**Required fix**:
- Check the error returned by `domain.NewPosition()`.
- If non-nil, return it as an internal server error (`http.StatusInternalServerError`) wrapped with context (e.g., `"new position: %w"`).
- Remove the stale comment above the call.

**Rationale**: `validateDepositRequest` currently enforces strictly-positive amounts, and `NewPosition` rejects only negative amounts, so the error is currently unreachable. The error must still be checked because preconditions can change independently.

### FR-2: Propagate `json.Marshal()` error in `deposit_service.go`

**Location**: `internal/application/deposit_service.go`, `Execute` method, ~line 113.

**Current code**:
```go
// DepositResponse contains only string fields; json.Marshal cannot fail.
snapshotBytes, _ := json.Marshal(resp)
```

**Required fix**:
- Check the error returned by `json.Marshal()`.
- If non-nil, return it as an internal server error (`http.StatusInternalServerError`) wrapped with context (e.g., `"marshal response snapshot: %w"`).
- Remove the stale comment above the call.

### FR-3: Surface rollback error in `transaction.go`

**Location**: `internal/adapters/outbound/postgres/transaction.go`, `Do` method, ~line 31.

**Current code**:
```go
if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
    _ = tx.Rollback(ctx)
    return err
}
```

**Required fix**:
- Check the error returned by `tx.Rollback()`.
- If rollback also fails, combine both errors using `errors.Join` (stdlib, Go 1.20+) so both the fn error and the rollback error are surfaced.
- If only the fn error occurred, return it unchanged (preserving current behavior for the common case).

## Non-Functional Requirements

- **NFR-1**: All changed executable lines must have test coverage. Each violation fix must include corresponding test cases that exercise both the success path and the newly propagated error path.
- **NFR-2**: No business behavior changes. The fixes only add error propagation on paths that currently discard errors. Happy-path behavior and HTTP status codes remain identical.
- **NFR-3**: Stale comments exposed by each fix must be removed per cleanup rules.

## Scope

### In Scope

- `internal/application/deposit_service.go` — fix violations 1 and 2, remove stale comments.
- `internal/adapters/outbound/postgres/transaction.go` — fix violation 3.
- `internal/application/deposit_service_test.go` — add/update tests for propagated errors from `NewPosition` and `json.Marshal`.
- `internal/adapters/outbound/postgres/transaction_test.go` — add/update test for rollback-failure error combining.

### Out of Scope

- Changing domain validation rules (`NewPosition`, `validateDepositRequest`).
- Changing HTTP handler behavior or response contracts.
- Changing the `UnitOfWork` port interface.
- Broad codebase-wide error-handling audit beyond the three identified violations.
- Adding new logging, metrics, or observability for these error paths.

## Constraints and Assumptions

- **Go version**: The codebase uses Go 1.20+ (required for `errors.Join`). Confirm from `go.mod`.
- **`errors.Join` semantics**: `errors.Join(fnErr, rollbackErr)` produces an error that unwraps to both, preserving `errors.Is` behavior for both constituent errors.
- **Current unreachability**: Violations 1 and 2 are currently unreachable due to upstream validation. The fixes must still be testable by constructing scenarios where the called functions return errors (via mocks for `deposit_service_test.go`, or test doubles for `transaction_test.go`).
- **Transaction test is integration**: `transaction_test.go` uses the `//go:build integration` tag. New rollback-failure tests in that file must follow the same convention.

## Existing Context

- `validateDepositRequest` enforces `amount.IsPositive()` and `unitPrice.IsPositive()` (strictly positive).
- `domain.NewPosition` rejects only `amount.IsNegative()` or `unitPrice.IsNegative()` (rejects negative, allows zero).
- The gap (zero is allowed by domain but rejected by service) means the domain error is indeed currently unreachable from the service, but this does not justify discarding it.
- `DepositResponse` contains only `string` fields, making `json.Marshal` failure currently unreachable, but this does not justify discarding it.
- `transaction.go` rollback is called only when `fn` already failed; rollback failure typically indicates a lost connection, which is rare but must be surfaced.

## Technical Details

### Error Wrapping Patterns

- **FR-1 and FR-2**: `fmt.Errorf("context: %w", err)` — consistent with existing wrapping in `deposit_service.go`.
- **FR-3**: `errors.Join(fnErr, rollbackErr)` when both fail; return `fnErr` unchanged when rollback succeeds. `errors.Join` preserves `errors.Is`/`errors.As` traversal for both constituent errors. No additional wrapping around `errors.Join` — the combined error speaks for itself.

### Test Strategy

**FR-3** (transaction.go): Add an integration test that forces rollback failure by cancelling the parent context inside `fn`. When `Do` attempts `tx.Rollback(ctx)` with the already-cancelled context, pgx returns a context error. Assert the returned error satisfies `errors.Is` for both the original `fn` error and `context.Canceled`.

**FR-1 and FR-2** (deposit_service.go): The error branches are unreachable from `Execute`'s public API. `validateDepositRequest` enforces strictly positive amounts, blocking all inputs that could fail `NewPosition` (which only rejects negative). `DepositResponse` has only string fields, making `json.Marshal` infallible for it. The assignment-line changes (`_ =` → named error) are covered by existing happy-path tests. The conditional branch bodies (the `return` statements) are a documented narrow coverage exception: they exist per error-handling/fail-fast rules, and the called functions are independently tested in their own packages.

## Technical Debt (Discovered, Deferred)

The following issues were found in in-scope files during audit but are not required for safe execution of this feature:

- **Discarded errors in test helpers** (`deposit_service_test.go`, `transaction_test.go`): Several test setup and cleanup calls use `_ =` or `_, _ =` to discard errors (e.g., `newTestClient()`, test cleanup `pool.Exec`, `json.Marshal` in test fixtures). Per strict error-handling rules these should be checked, but they are test setup/cleanup code and do not affect production correctness. Deferring to a separate test-hygiene task.

## Open Questions

None. Go version is 1.25 (`go.mod`), confirming `errors.Join` is available for FR-3.

## Success Criteria

1. `grep -rn '_ =' internal/` returns zero matches for discarded error returns in the touched files.
2. All three stale justification comments are removed.
3. Unit/integration tests exist for each newly propagated error path.
4. All existing tests continue to pass with no behavior change.
5. Test coverage on changed lines is 100%.
