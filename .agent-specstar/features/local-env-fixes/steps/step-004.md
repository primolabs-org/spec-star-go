# Step 004 - Verify `AGENTS.md` matches the locked baseline

## Metadata
- Feature: local-env-fixes
- Step: step-004
- Status: pending
- Depends On: step-003
- Last Updated: 2026-04-19

## Objective

Verify that `.github/agents/AGENTS.md` already matches the locked baseline defined by FR-20 of `design.md` and correct any drift in place. Most of `AGENTS.md` was previously patched and is expected to require no changes; this step exists to confirm that and to capture the verification result.

## In Scope

- Read `.github/agents/AGENTS.md` end-to-end.
- For each FR-20 sub-item, confirm the file says what `design.md` requires.
- If any sub-item drifted, correct it with the smallest possible edit.
- Record the verification outcome in the completion report, item-by-item.

## Out of Scope

- Restructuring `AGENTS.md`.
- Moving `AGENTS.md` to a different location.
- Adding new sections beyond what FR-20 requires.
- Editing `Dockerfile`, `docker-compose.yml`, `.golangci.yml`, `go.mod`, `go.sum`, `.env.example`, or any Go source.
- Editing other instruction files under `.github/instructions/`.

## Required Reads

- `.agent-specstar/features/local-env-fixes/design.md` (section "`AGENTS.md` verification" and FR-20)
- `.github/agents/AGENTS.md`
- `.github/instructions/cleanup.instructions.md`

## Allowed Write Paths

- `.github/agents/AGENTS.md` (only if drift is found)

## Forbidden Paths

- everything else

## Known Abstraction Opportunities

- none

## Allowed Abstraction Scope

None.

## Required Tests

No automated tests. Verification is documented in the completion report as a per-FR-20-item checklist with quoted excerpts from `AGENTS.md` proving each item is satisfied.

## Coverage Requirement

Not applicable.

## Failure Model

Fail-fast. If the file is missing, do not regenerate it from scratch in this step — escalate. If a sub-item is materially absent (not just worded differently), correct it; do not leave it for "later".

## Allowed Fallbacks

- none

## Acceptance Criteria

- `.github/agents/AGENTS.md` satisfies every sub-item of FR-20:
  - repository purpose describes a fixed-income wallet position service (not valuation)
  - Go 1.26.2 is the pinned toolchain version
  - builder image is pinned to `golang:1.26.2-alpine`
  - local harness is Docker Compose + PostgreSQL + RIE-based local Lambda execution
  - PostgreSQL must define a non-empty `POSTGRES_PASSWORD`
  - `golangci-lint` requires a repository-owned configuration
  - validation commands include `go build ./...`
  - no remaining placeholder/template sections (no `<...>`, `TODO`, `TBD`, `XXX`, or `placeholder` markers)
- Completion report contains a per-item checklist with the exact line(s) from `AGENTS.md` proving each item, plus a summary of any corrections made.

## Deferred Work

- none

## Escalation Conditions

- `.github/agents/AGENTS.md` is missing.
- A sub-item is materially absent and cannot be corrected with a small, local edit (would require restructuring the file).
- Drift in `AGENTS.md` reveals a contradiction with `design.md` itself.
