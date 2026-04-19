# Step 001 - Add repository-owned `.golangci.yml`

## Metadata
- Feature: local-env-fixes
- Step: step-001
- Status: pending
- Depends On: none
- Last Updated: 2026-04-19

## Objective

Add a checked-in `.golangci.yml` at the repository root that becomes the source of truth for `golangci-lint` behavior, satisfying FR-4 through FR-9 of `design.md`.

## In Scope

- Create `.golangci.yml` at the repository root.
- Confirm the locally installed `golangci-lint` is on the v2 major series.
- Run `golangci-lint run ./...` against the existing codebase to confirm the configuration loads cleanly.

## Out of Scope

- Fixing any lint findings the new configuration surfaces. Findings are recorded in the completion report. Fixes, if needed, are addressed in a follow-up task or in step-002 only when they sit on lines the toolchain bump already touches.
- Adding linters beyond the required minimum.
- Editing any Go source file.
- Editing `.github/agents/AGENTS.md`.

## Required Reads

- `.agent-specstar/features/local-env-fixes/design.md` (sections "`.golangci.yml`" and "Failure Model")
- `.github/agents/AGENTS.md` (sections 7 and 8)
- `.github/instructions/cleanup.instructions.md`
- `.github/instructions/fail-fast.instructions.md`

## Allowed Write Paths

- `.golangci.yml`

## Forbidden Paths

- everything else

## Known Abstraction Opportunities

- none

## Allowed Abstraction Scope

None. The configuration must stay minimal and literal.

## Required Tests

- `golangci-lint --version` reports a v2.x.y version.
- `golangci-lint config verify` (or equivalent for the installed version) reports no schema errors.
- `golangci-lint run ./...` exits with either code `0` (no findings) or a non-zero code that lists real findings, never with a configuration error.

No Go test files are added or changed in this step.

## Coverage Requirement

Not applicable. No executable lines change.

## Failure Model

Fail-fast. If the installed `golangci-lint` is not v2, stop and surface the mismatch as a blocker. Do not downgrade the schema to match an older binary.

## Allowed Fallbacks

- none

## Acceptance Criteria

- `.golangci.yml` exists at the repository root.
- The file declares `version: "2"`.
- The `linters` block sets `default: none` and enables exactly: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`. Additional linters MAY be added only if a single-line YAML comment justifies each one.
- No `linters.disable`, `issues.exclude-rules`, or per-file overrides remove any of the five required linters.
- `golangci-lint run ./...` runs and reports either zero findings or real findings, with no configuration errors.
- Completion report lists the installed `golangci-lint` version, the exact command outputs, and any findings produced by the new configuration.

## Deferred Work

- Fixing lint findings produced by the new configuration is deferred to follow-up work unless the findings sit on lines later steps touch.

## Escalation Conditions

- Installed `golangci-lint` is not v2.
- The configuration cannot be loaded by the installed binary.
- `golangci-lint run ./...` reports a configuration error rather than findings.
