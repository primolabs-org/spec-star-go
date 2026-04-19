# Step 002 Review — HTTP handlers: LoggerFactory injection, terminal error logging, bootstrap migration

**Verdict: APPROVED**

## Checklist

| # | Criterion | Result |
|---|---|---|
| 1 | `loggerFactory` interface defined in `httphandler` package with correct signature | PASS |
| 2 | `loggers loggerFactory` field added to both handler structs | PASS |
| 3 | Both constructors accept `loggerFactory` parameter | PASS |
| 4 | `Handle` creates per-request logger, stores in context via `platform.WithLogger`, passes enriched ctx to service | PASS |
| 5 | Terminal error logging: ERROR for 5xx, WARN for 4xx, with `status`, `error`, `outcome` fields | PASS |
| 6 | `main.go`: `LoggerFactory` created, `log.Fatalf` replaced with `slog.Error` + `os.Exit(1)`, factory passed to handlers | PASS |
| 7 | Scope: no changes to application services, outbound adapters, domain, or ports | PASS |
| 8 | Test quality: mock logger factory with buffer-backed JSON handler, new tests for error/warn log paths, existing tests updated | PASS |
| 9 | Coverage: 100% on all changed handler files | PASS |
| 10 | Clean code: no dead code, no stale comments, no unused imports | PASS |
| 11 | Error handling: errors not discarded, logging does not affect request processing | PASS |

## Scope verification

Changed files (unstaged diff):
- `cmd/http-lambda/main.go` — allowed
- `internal/adapters/inbound/httphandler/deposit_handler.go` — allowed
- `internal/adapters/inbound/httphandler/deposit_handler_test.go` — allowed
- `internal/adapters/inbound/httphandler/withdraw_handler.go` — allowed
- `internal/adapters/inbound/httphandler/withdraw_handler_test.go` — allowed
- `.agent-specstar/features/logging/feature-state.json` — workflow artifact

No forbidden paths touched. No changes to `internal/application/`, `internal/adapters/outbound/`, `internal/domain/`, `internal/ports/`, or `internal/platform/logger.go`.

## Implementation quality

- **`loggerFactory` interface**: Defined once in `deposit_handler.go`, shared across the package. Correct consumer-side abstraction per hexagonal architecture.
- **`logTerminalError` helper**: Defined once in `deposit_handler.go`, reused by `withdraw_handler.go`. Clean shared utility within the package. Correct level dispatch (5xx → ERROR, <5xx → WARN). Stable field model (`status`, `error`, `outcome`).
- **Logger placement in Handle**: Logger is created after body parsing but before service call. Body parse failures and method-not-allowed are correctly excluded from logging per the step spec's out-of-scope section.
- **Context propagation**: `platform.WithLogger(ctx, logger)` is called before `service.Execute(ctx, ...)`. Tests verify the enriched context reaches the service mock via `capturedCtx`.
- **main.go migration**: `"log"` import removed, `"log/slog"` and `"os"` added. `LoggerFactory` created before config loading so `slog.SetDefault` is active for startup error logs. Both `log.Fatalf` calls replaced with `slog.Error(...)` + `os.Exit(1)`.

## Test quality

- **Mock**: `mockLoggerFactory` uses `bytes.Buffer`-backed `slog.JSONHandler`, enabling structured assertion on log output fields. `parseLastEntry` helper parses the last JSON log line.
- **Deposit handler tests**: 5xx → ERROR level, 4xx → WARN level, context-with-logger propagation.
- **Withdraw handler tests**: 5xx → ERROR level, `ErrInsufficientPosition` (409) → WARN, `ErrConcurrencyConflict` (409) → WARN, context-with-logger propagation.
- **All existing tests updated** with `newMockLoggerFactory()` in constructor calls and continue to pass.

## Verification results

- `go test ./internal/adapters/inbound/httphandler/... -v -count=1` — **all 30 tests PASS**
- `go test ./... -count=1` — **all packages PASS**
- `go vet ./...` — **clean**
- `go build ./cmd/http-lambda/` — **compiles**
- Coverage on handler files — **100.0% of statements**

## Notes

No issues found. Implementation is precise, well-scoped, and matches the step contract exactly.
