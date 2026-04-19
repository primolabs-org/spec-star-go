# Step 007 — README update and end-to-end validation

## Metadata
- Feature: tracing
- Step: step-007
- Status: pending
- Depends On: step-001, step-002, step-003, step-004, step-005, step-006
- Last Updated: 2026-04-19

## Objective

Update the repository README with complete observability documentation and perform end-to-end validation that the full stack works as designed. Update `e2e-test.sh` if needed to reflect the `lambda` → `wallet-api` service rename.

## In Scope

- Modify `README.md` to document:
  - Updated local stack services and their ports.
  - How to start the stack (`docker compose up --build`).
  - Grafana URL (`http://localhost:3000`) and how to browse logs in Grafana Explore using the Loki data source.
  - Jaeger URL (`http://localhost:16686`) and how to inspect traces.
  - Example `curl` commands for deposit and withdrawal invocations (verify existing instructions still work after the service rename).
  - Which environment variables control OTel export (`OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`).
  - Known limitations: Docker socket access for Alloy on different host OSes, local-only stack not suitable for production, metrics visible via collector debug logs only (no Prometheus/Mimir backend).
- Modify `e2e-test.sh` if it references the old `lambda` service name.
- Perform end-to-end validation (manual or scripted):
  1. `docker compose up --build` starts all 7 services.
  2. Deposit request produces a trace in Jaeger with root and child spans.
  3. Withdrawal request produces a trace in Jaeger with root and child spans.
  4. Structured JSON logs with `trace_id` and `span_id` appear in Grafana Explore.
  5. `docker compose logs wallet-api` shows structured JSON logs.
  6. Application returns correct deposit and withdrawal responses.

## Out of Scope

- Any Go application code changes.
- Any Docker Compose or config file changes (unless fixing issues found during validation).
- Creating new observability dashboards in Grafana.

## Required Reads

- `.agent-specstar/features/tracing/design.md` — FR-13, README requirements, Success Criteria, Validation requirements.
- `README.md` — current content.
- `e2e-test.sh` — current script.
- `docker-compose.yml` — final service names and ports.

## Allowed Write Paths

- `README.md` (MODIFY)
- `e2e-test.sh` (MODIFY — if service name references need updating)

## Forbidden Paths

- `cmd/**`
- `internal/**`
- `docker-compose.yml`
- `local-env/**`
- `Dockerfile`

## Known Abstraction Opportunities

- None.

## Allowed Abstraction Scope

- None.

## Required Tests

No Go unit tests for this step.

End-to-end validation checklist (manual):
1. `docker compose up --build` starts `postgres`, `wallet-api`, `otel-collector`, `jaeger`, `loki`, `alloy`, `grafana` without errors.
2. Deposit request via curl produces a 201 response with correct body.
3. The trace for the deposit is visible in Jaeger at `http://localhost:16686` with spans: `POST /deposits`, `deposit.execute`, and `db.*` child spans.
4. Withdrawal request via curl produces a 200 response with correct body.
5. The trace for the withdrawal is visible in Jaeger with spans: `POST /withdrawals`, `withdraw.execute`, and `db.*` child spans.
6. In Grafana Explore (`http://localhost:3000`), selecting the Loki data source and querying for `{container=~".*wallet-api.*"}` shows structured JSON logs.
7. Log records include `trace_id` and `span_id` fields.
8. `docker compose logs wallet-api` shows structured JSON output with trace correlation fields.
9. `e2e-test.sh` runs successfully (if applicable).

## Coverage Requirement

N/A — no Go code changes.

## Failure Model

- If validation reveals issues in previous steps, those steps must be revisited. Do not paper over problems in the README.

## Allowed Fallbacks

- none

## Acceptance Criteria

1. README contains complete observability documentation as specified in the design.
2. All example commands in the README are verified to work.
3. End-to-end validation checklist passes.
4. `e2e-test.sh` works with the renamed service.

## Deferred Work

- none

## Escalation Conditions

- If end-to-end validation reveals a critical issue in a previous step, escalate back to that step for a fix rather than working around it in the README.
