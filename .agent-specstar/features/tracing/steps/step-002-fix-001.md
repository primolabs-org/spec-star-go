# Step 002 Fix 001 - Runtime trace-log correlation in active spans

## Metadata
- Feature: tracing
- Step: step-002-fix-001
- Status: pending
- Depends On: step-007
- Trigger: review rejection in `reviews/step-007-review.md`

## Objective
Fix the log-trace correlation regression discovered during step-007 validation so runtime logs emitted during active spans contain `trace_id` and `span_id`.

## In Scope
- Update contextless logging calls in tracing-instrumented runtime paths to context-aware calls so logger handlers receive active span context.
- Apply this in inbound HTTP terminal error logging and outbound Postgres error logging paths used during request processing.
- Keep existing logger architecture (`slog` JSON -> stdout + `traceContextHandler`) unchanged.
- Add focused regression tests proving correlation fields are present when a recording span is active in context for these runtime paths.

## Out of Scope
- README, Docker Compose, and local-env changes.
- Any changes to span names, span attributes, or metric names.
- Any redesign of logger factory interfaces.
- Domain behavior changes.

## Allowed Write Paths
- `internal/adapters/inbound/httphandler/deposit_handler.go`
- `internal/adapters/inbound/httphandler/withdraw_handler.go`
- `internal/adapters/inbound/httphandler/deposit_handler_test.go`
- `internal/adapters/inbound/httphandler/withdraw_handler_test.go`
- `internal/adapters/outbound/postgres/asset_repository.go`
- `internal/adapters/outbound/postgres/client_repository.go`
- `internal/adapters/outbound/postgres/position_repository.go`
- `internal/adapters/outbound/postgres/processed_command_repository.go`
- `internal/adapters/outbound/postgres/transaction.go`
- `internal/adapters/outbound/postgres/logging_test.go`

## Forbidden Paths
- `README.md`
- `docker-compose.yml`
- `e2e-test.sh`
- `internal/platform/logger.go`
- `internal/platform/otel.go`
- `internal/application/**`
- `internal/domain/**`
- `cmd/**`

## Required Validation
1. Run:
   - `go test ./internal/adapters/inbound/httphandler/...`
   - `go test ./internal/adapters/outbound/postgres/... -run TestLogging`
2. Re-run step-007 manual checklist items related to logs:
   - Grafana Explore query for `wallet-api` shows structured JSON logs.
   - Log records include `trace_id` and `span_id`.
   - `docker compose logs wallet-api` shows structured JSON logs with correlation fields.
3. Confirm no edits outside allowed write paths.

## Acceptance Criteria
1. Runtime error logs emitted in active trace contexts include non-empty `trace_id` and `span_id` fields.
2. Existing log field model (`service`, `trigger`, `operation`, etc.) remains intact.
3. Regression tests for inbound and outbound logging correlation pass.
4. Step-007 log-correlation validation items pass without README workaround.
5. No edits outside allowed write paths.
