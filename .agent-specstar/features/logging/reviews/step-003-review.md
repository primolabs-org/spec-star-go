# Step 003 Review — Application services: operation outcome and idempotency logging

**Verdict: APPROVED**

**Reviewed files:**
- `internal/application/deposit_service.go`
- `internal/application/withdraw_service.go`
- `internal/application/deposit_service_test.go`
- `internal/application/withdraw_service_test.go`

## Checklist

| # | Check | Result |
|---|---|---|
| 1 | Logger retrieval via `platform.LoggerFromContext(ctx)`, no constructor changes | PASS |
| 2 | Deposit idempotency replay: INFO with `order_id`, `outcome=replayed` | PASS |
| 3 | Deposit race condition replay: INFO with `order_id`, `outcome=replayed` | PASS |
| 4 | Deposit infra failures: ERROR with `error`, `outcome=failed`, entity IDs | PASS |
| 5 | Deposit 4xx paths: no logging for client not found, asset not found, invalid product type | PASS |
| 6 | Withdraw: same patterns — replay INFO, infra ERROR, no 4xx logging | PASS |
| 7 | Scope: no changes to handlers, adapters, domain, ports, or constructors | PASS |
| 8 | `fmt.Errorf` calls preserved unchanged | PASS |
| 9 | Test quality: buffer-backed JSON handler, presence/absence assertions, field-specific checks | PASS |
| 10 | Coverage: 100% on changed lines | PASS |
| 11 | Clean code: no dead code, stale comments, unused imports | PASS |

## Scope compliance

Only allowed files were modified:
- `internal/application/deposit_service.go` — added `platform` import, logger retrieval, 4 ERROR logs, 2 INFO logs
- `internal/application/withdraw_service.go` — added `platform` import, logger retrieval, 4 ERROR logs, 2 INFO logs
- `internal/application/deposit_service_test.go` — added `parseLogEntries` helper, 8 logging tests
- `internal/application/withdraw_service_test.go` — added 9 logging tests (uses shared `parseLogEntries`)

No forbidden paths were touched.

## Implementation quality

**Log emission points match the design exactly:**
- 5xx infrastructure failures → `logger.ErrorContext` with `error`, `outcome=failed`, and relevant entity identifiers (`client_id`, `asset_id`, `order_id`)
- Idempotency replays → `logger.InfoContext` with `order_id`, `outcome=replayed`
- Race condition replays → `logger.InfoContext` with `order_id`, `outcome=replayed`
- 4xx business outcomes → no logging (correct)

**Field model is consistent** across both services — `error`, `outcome`, `order_id`, `client_id`, `asset_id` used appropriately.

**`ErrorContext`/`InfoContext` used correctly** — passes ctx for any future handler-level enrichment.

## Test quality

Tests use the correct pattern: `slog.New(slog.NewJSONHandler(&buf, nil))` with `platform.WithLogger(ctx, logger)`.

**Presence tests verify:** log level, `outcome` field value, `order_id` value, entity identifiers, `error` field existence.

**Absence tests verify:** zero buffer output for client not found, asset not found (deposit); client not found, no positions, insufficient position, concurrency conflict (withdraw).

## Coverage

- `withdraw_service.go` — 100% on all functions
- `deposit_service.go:Execute` — 95.7% function-level; the two uncovered branches are pre-existing (`NewPosition` error at line 112, `json.Marshal` error at line 119), not introduced by this step. All lines added by step-003 are covered.

## Verification results

- `go test ./internal/application/... -v -count=1` — all tests PASS
- `go test ./... -count=1` — all packages PASS, no regressions
- `go vet ./...` — clean, no issues

## Notes

Pre-existing tests that hit logging paths without providing a logger via context now emit text-format log lines to stderr (via `slog.Default()` fallback). This is expected per the design's explicit fallback decision and does not affect test correctness.
