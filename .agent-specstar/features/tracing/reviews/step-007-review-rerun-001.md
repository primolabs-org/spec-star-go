# Step 007 Review Rerun 001 - tracing

## Metadata
- Feature: tracing
- Step: step-007
- Reviewer mode: SpecStar Reviewer
- Date: 2026-04-21
- Trigger: approved remediation in `.agent-specstar/features/tracing/reviews/step-002-fix-001-review.md`

## Verdict
APPROVED

## Review Scope
- Step contract: `.agent-specstar/features/tracing/steps/step-007.md`
- Design context: `.agent-specstar/features/tracing/design.md` (FR-13, Success Criteria #1-#5)
- Prior rejection: `.agent-specstar/features/tracing/reviews/step-007-review.md`
- Remediation evidence: `.agent-specstar/features/tracing/reviews/step-002-fix-001-review.md`
- Current workspace state and rerun validation commands

## Key Findings

### 1) Prior blocking issue is remediated and remains in place
The rejected condition (missing log-trace correlation in runtime error paths) is fixed in current code and covered by regression tests.

Implementation evidence:
- Context-aware terminal logging in inbound handlers:
  - `internal/adapters/inbound/httphandler/deposit_handler.go:77`
  - `internal/adapters/inbound/httphandler/deposit_handler.go:84`
  - `internal/adapters/inbound/httphandler/deposit_handler.go:87`
  - `internal/adapters/inbound/httphandler/withdraw_handler.go:59`
- Context-aware terminal logging in outbound Postgres paths:
  - `internal/adapters/outbound/postgres/processed_command_repository.go:57`
  - `internal/adapters/outbound/postgres/processed_command_repository.go:87`
  - `internal/adapters/outbound/postgres/transaction.go:35`
  - `internal/adapters/outbound/postgres/transaction.go:55`

Regression test evidence:
- Inbound correlation tests:
  - `internal/adapters/inbound/httphandler/deposit_handler_test.go:458`
  - `internal/adapters/inbound/httphandler/withdraw_handler_test.go:371`
- Outbound correlation tests:
  - `internal/adapters/outbound/postgres/logging_test.go:564`
  - `internal/adapters/outbound/postgres/logging_test.go:669`
  - `internal/adapters/outbound/postgres/logging_test.go:714`

Validation rerun:
- `go test ./internal/adapters/inbound/httphandler/...` -> PASS
- `go test ./internal/adapters/outbound/postgres/... -run TestLogging` -> PASS

Remediation review corroboration:
- Verdict approved at `.agent-specstar/features/tracing/reviews/step-002-fix-001-review.md:11`
- Runtime correlation checks already documented at `.agent-specstar/features/tracing/reviews/step-002-fix-001-review.md:61`
- Step-007 log-correlation checklist items marked PASS at `.agent-specstar/features/tracing/reviews/step-002-fix-001-review.md:74`

### 2) End-to-end checklist items related to `trace_id`/`span_id` are satisfied
Step-007 requires:
- Grafana/Loki logs with `trace_id` and `span_id`: `.agent-specstar/features/tracing/steps/step-007.md:29`, `.agent-specstar/features/tracing/steps/step-007.md:78`
- `docker compose logs wallet-api` showing structured trace-correlated logs: `.agent-specstar/features/tracing/steps/step-007.md:30`, `.agent-specstar/features/tracing/steps/step-007.md:79`

Rerun evidence in current workspace:
- `docker compose up -d --build` started all seven services (`postgres`, `wallet-api`, `otel-collector`, `jaeger`, `loki`, `alloy`, `grafana`).
- Deposit response check returned `statusCode=201`.
- Withdrawal response check returned `statusCode=200`.
- Jaeger API checks returned traces with required child spans:
  - `deposit_traces=2 deposit_execute=true deposit_db_child=true`
  - `withdraw_traces=1 withdraw_execute=true withdraw_db_child=true`
- Compose logs check returned correlation fields:
  - `compose_json_lines=22 trace_id_hits=1 span_id_hits=1`
- Loki query with checklist selector `{container=~".*wallet-api.*"}` returned correlated logs:
  - `loki_streams=3 correlated_entries=1`

### 3) README and script requirements are satisfied
README contains required observability documentation and command guidance:
- OTel environment variables: `README.md:82`
- Seven-service stack and startup docs: `README.md:122`
- Grafana Explore instructions and Loki selector: `README.md:184`
- Jaeger trace inspection instructions: `README.md:197`
- Known limitations section: `README.md:208`
- Service-rename compatibility statement for examples: `README.md:224`

`e2e-test.sh` compatibility and execution:
- Uses compose up/down workflow without old `lambda` service-name references: `e2e-test.sh:7`, `e2e-test.sh:37`
- Executed in this rerun and completed with:
  - `PASS: deposit`
  - `PASS: withdrawal`
  - `All tests passed.`

## Acceptance Criteria Check
1. README contains complete observability documentation as specified: PASS
2. All example commands in the README are verified to work: PASS
3. End-to-end validation checklist passes: PASS
4. `e2e-test.sh` works with the renamed service: PASS

## Outcome
Step `step-007` is approved for completion.
