# Step 007 Review - tracing

## Metadata
- Feature: tracing
- Step: step-007
- Reviewer mode: SpecStar Reviewer
- Date: 2026-04-21

## Verdict
REJECTED

## Review Scope
- Step contract: `.agent-specstar/features/tracing/steps/step-007.md`
- Design context: `.agent-specstar/features/tracing/design.md` (FR-13, Success Criteria #1-#5)
- Implementation under review: `README.md` update and validation claim for step-007
- Additional evidence input: engineer reported missing `trace_id`/`span_id` in logs during end-to-end validation

## Findings

### 1) High - End-to-end validation checklist did not pass
Step-007 requires the validation checklist to pass, including log-trace correlation fields in Grafana and compose logs.

Blocking evidence:
- Engineer-reported validation result: missing `trace_id`/`span_id` in logs.
- Step-007 checklist explicitly requires these fields:
  - `.agent-specstar/features/tracing/steps/step-007.md:78`
  - `.agent-specstar/features/tracing/steps/step-007.md:79`
- Acceptance criterion "End-to-end validation checklist passes" is therefore not met:
  - `.agent-specstar/features/tracing/steps/step-007.md:98`

Corroborating implementation evidence indicates this is a prior-step regression, not a README issue:
- Trace correlation injection is context-driven in logger handler:
  - `internal/platform/logger.go:24`
  - `internal/platform/logger.go:25`
  - `internal/platform/logger.go:26`
- Multiple runtime error logs in instrumented paths are emitted with contextless logger calls (which drop span context):
  - `internal/adapters/inbound/httphandler/deposit_handler.go:84`
  - `internal/adapters/inbound/httphandler/deposit_handler.go:87`
  - `internal/adapters/outbound/postgres/processed_command_repository.go:57`
  - `internal/adapters/outbound/postgres/transaction.go:35`

This aligns with step-007 escalation conditions: validation surfaced a critical issue from previous tracing work and must be fixed there, not papered over in README.

### 2) Medium - No future step currently captures remediation
There is no planned future tracing step beyond step-007 in `.agent-specstar/features/tracing/steps/` that explicitly resolves the correlation failure, and step-007 declares no deferred work.

Without an explicit fix-step, the current rejection condition would become hidden deferred work.

## Acceptance Criteria Check
1. README contains complete observability documentation as specified: PASS
   - Evidence sections present in `README.md`: OpenTelemetry vars, 7-service stack, Grafana/Jaeger usage, known limitations (`README.md:82`, `README.md:122`, `README.md:184`, `README.md:197`, `README.md:208`).
2. Example commands in README are verified to work: PARTIAL
   - Deposit/withdraw usage is documented and aligned with current stack (`README.md:224`), but full verification cannot be accepted while required checklist items fail.
3. End-to-end validation checklist passes: FAIL
   - Reported missing `trace_id`/`span_id` directly violates required checklist items and AC #3.
4. `e2e-test.sh` works with renamed service: PASS (no `docker compose logs lambda`/old service reference in script path; no rename breakage observed in script content).

## Scope Compliance
- Product-file edits for this step are within intended documentation scope (`README.md` updated).
- `e2e-test.sh` was not modified, which is acceptable because old service-name references were not present.
- Workflow metadata change in `.agent-specstar/features/tracing/feature-state.json` is non-product and not a rejection trigger.

## Required Outcome
Step-007 is rejected until trace-log correlation failure is fixed in a focused remediation step and the end-to-end checklist is re-run successfully.
