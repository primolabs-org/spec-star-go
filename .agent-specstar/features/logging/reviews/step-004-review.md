# Step 004 Review — Outbound adapters: dependency failure logging

**Verdict: APPROVED**

**Reviewer:** SpecStar Reviewer  
**Date:** 2025-04-19

## Summary

The implementation correctly adds structured ERROR-level logging for infrastructure-level database failures across all outbound PostgreSQL adapters, with well-structured unit tests that verify both log presence and absence.

## Checklist

| # | Check | Result |
|---|---|---|
| 1 | Logger retrieval via `platform.LoggerFromContext(ctx)` — no constructor changes | ✅ Pass |
| 2 | ClientRepository: ERROR for unexpected errors, NO log for ErrNoRows | ✅ Pass |
| 3 | AssetRepository: ERROR for unexpected errors, NO log for ErrNoRows | ✅ Pass |
| 4 | PositionRepository: ERROR for unexpected errors, NO log for ErrNoRows/ErrConcurrencyConflict | ✅ Pass |
| 5 | ProcessedCommandRepository: ERROR for unexpected errors, NO log for ErrNoRows/ErrDuplicate | ✅ Pass |
| 6 | TransactionRunner: ERROR for Begin/Commit failures, NO log for Rollback failures | ✅ Pass |
| 7 | Forbidden paths untouched (helpers.go, testhelper_test.go, existing *_test.go) | ✅ Pass |
| 8 | Scope: changes only in `internal/adapters/outbound/postgres/` | ✅ Pass (see note 1) |
| 9 | Test quality: mock DBTX with buffer-backed JSON handler, presence + absence | ✅ Pass |
| 10 | logging_test.go: no `//go:build integration` tag | ✅ Pass |
| 11 | Error handling: no errors discarded, `fmt.Errorf` wrapping preserved | ✅ Pass |
| 12 | Clean code: no dead code, stale comments, unused imports | ✅ Pass |
| 13 | `beginner` interface: minimal, justified, public API unchanged | ✅ Pass |

## Implementation Details

### Production code

- **client_repository.go**: Adds `platform` import. `FindByID` logs after ErrNoRows guard. `Create` logs on exec error. Both include `client_id` and `error` fields.
- **asset_repository.go**: Adds `platform` import. `FindByID` and `FindByInstrumentID` restructured from direct `return scanAsset(...)` to capture error and log when not `domain.ErrNotFound`. `Create` logs on exec error. All include entity identifiers.
- **position_repository.go**: Adds `platform` import. `FindByID` logs after ErrNoRows guard. `FindByClientAndAsset` and `FindByClientAndInstrument` log on query error. `Create` logs on exec error. `Update` logs on exec error but NOT on zero rows affected (ErrConcurrencyConflict). All include entity identifiers.
- **processed_command_repository.go**: Adds `platform` import. `FindByTypeAndOrderID` logs after ErrNoRows guard. `Create` checks unique violation first (no log), then logs for unexpected errors. Fields include `command_type`, `order_id`, `command_id`.
- **transaction.go**: Introduces `beginner` interface (single-method, unexported) to enable testing. `pool` field type changes from `*pgxpool.Pool` to `beginner`. `NewTransactionRunner` still accepts `*pgxpool.Pool`. `Do` logs ERROR for Begin and Commit failures. Rollback failure is not logged (already joined via `errors.Join`).

### `beginner` interface justification

The `beginner` interface is minimal (one method), unexported, and necessary because `TransactionRunner` tests need to mock Begin behavior without a real connection pool. `NewTransactionRunner` public constructor signature is unchanged (`*pgxpool.Pool`), so no callers are affected.

### Test file (logging_test.go)

- No `//go:build integration` tag — runs as fast unit tests.
- Uses `mockTx` (embeds `pgx.Tx` for interface satisfaction) with functional stubs.
- Uses `mockRow` implementing `pgx.Row` for QueryRow return values.
- Uses `mockBeginner` for TransactionRunner tests.
- Uses `bytes.Buffer`-backed `slog.JSONHandler` with `platform.WithLogger` for log capture.
- Parses JSON log output with `json.Decoder` to assert structured fields.
- All 17 required test cases present and passing:
  - ClientRepository: 3 tests (2 presence, 1 absence)
  - AssetRepository: 5 tests (3 presence, 2 absence) — exceeds required (includes FindByInstrumentID coverage)
  - PositionRepository: 7 tests (5 presence, 2 absence) — exceeds required (includes FindByClientAndAsset, FindByClientAndInstrument)
  - ProcessedCommandRepository: 4 tests (2 presence, 2 absence)
  - TransactionRunner: 2 tests (2 presence)

## Verification Results

| Command | Result |
|---|---|
| `go test ./internal/adapters/outbound/postgres/... -v -count=1 -run "TestLog"` | ✅ All 21 tests pass |
| `go test ./... -count=1` | ✅ All packages pass |
| `go vet ./...` | ✅ Clean |

## Notes

1. **`cover_detail.html` in diff**: A generated coverage report HTML file appears in the unstaged diff. This is not a step-004 code change but a leftover build artifact. It should be added to `.gitignore` or removed before commit. Not a blocking issue for this step.

## Conclusion

The implementation is clean, well-tested, and faithfully follows the step-004 specification. All acceptance criteria are met. No fix-step required.
