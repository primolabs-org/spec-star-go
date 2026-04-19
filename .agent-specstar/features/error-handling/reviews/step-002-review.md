# Step 002 Review — Propagate NewPosition and json.Marshal errors in deposit_service.go

## Verdict: approved

## Checklist

| # | Criterion | Status | Notes |
|---|-----------|--------|-------|
| 1 | `position, _ :=` replaced with `position, err :=` + error check returning 500 | Pass | Exact match with step definition |
| 2 | `snapshotBytes, _ :=` replaced with `snapshotBytes, err :=` + error check returning 500 | Pass | Exact match with step definition |
| 3 | Stale justification comment above `NewPosition` removed | Pass | `grep` confirms zero matches |
| 4 | Stale justification comment above `json.Marshal` removed | Pass | `grep` confirms zero matches |
| 5 | `grep '_ =' internal/application/deposit_service.go` returns zero matches | Pass | Exit code 1, no output |
| 6 | All existing tests pass unchanged | Pass | 27/27 tests pass |
| 7 | No forbidden paths touched | Pass | Only `internal/application/deposit_service.go` modified |
| 8 | No out-of-scope changes | Pass | Diff is minimal: exactly the two replacements specified |
| 9 | Test file untouched | Pass | No diff in `deposit_service_test.go` |

## Scope compliance

The diff modifies exactly two code blocks in `internal/application/deposit_service.go`:
- Lines 104-105 → 104-108: `domain.NewPosition` error propagation
- Lines 109-110 → 112-116: `json.Marshal` error propagation

No other lines were touched. No forbidden paths (`internal/domain/`, `internal/ports/`, `internal/adapters/`, `internal/platform/`) were modified.

## Error handling compliance

Both error returns follow the required pattern:
- `return nil, http.StatusInternalServerError, fmt.Errorf("<context>: %w", err)`
- Error messages are specific and preserve the original cause via `%w` wrapping

## Coverage

The assignment lines (`position, err := ...` and `snapshotBytes, err := ...`) are exercised by the two happy-path tests (`TestExecute_ValidDeposit_Returns201`, `TestExecute_ValidDeposit_ResponseFieldsMatchPosition`). The error branch bodies are unreachable from the public API, which is an accepted narrow coverage exception documented in the step definition.

## Findings

None.
