# Step 004 - End-to-End Test Script

## Metadata
- Feature: local-env
- Step: step-004
- Status: pending
- Depends On: step-003
- Last Updated: 2026-04-19

## Objective

Author `e2e-test.sh` at the repository root so that running `./e2e-test.sh` brings the local environment up, waits for both services to be ready, executes one deposit and one withdrawal against the seeded data per `.agent-specstar/features/local-env/design.md` § E2E scenario values, asserts each invocation's embedded `statusCode`, prints clear per-step pass/fail output, exits zero on success and non-zero on any failure, and unconditionally tears the environment down via a shell trap.

## In Scope

- New executable script `e2e-test.sh` at the repository root implementing the readiness, invocation, assertion, and teardown contract from the design.
- The script must `chmod +x`-equivalent (created with `0755` permissions) so `./e2e-test.sh` is directly executable.

## Out of Scope

- Compose, Dockerfile, SQL, README — produced by other steps.
- Any change to Go source, `go.mod`, `go.sum`, or `migrations/`.
- Adding negative test cases, parallel runs, fuzzing, performance assertions, or CI integration.

## Required Reads

- `.agent-specstar/features/local-env/design.md` — sections: Functional Requirements (FR-24 through FR-34), Invocation payload shape, Environment variables, Seed data, E2E scenario values, E2E readiness waits, E2E status assertion, Failure Model.
- `docker-compose.yml` (produced by step 003) — confirms service names and port defaults.
- `.env.example` (produced by step 003) — confirms env var names the script must source.

## Allowed Write Paths

- `e2e-test.sh`

## Forbidden Paths

- `cmd/**`
- `internal/**`
- `go.mod`, `go.sum`
- `migrations/**`
- `Dockerfile`, `.dockerignore`
- `docker-compose.yml`
- `.env`, `.env.example`
- `local-env/**`
- `README.md`
- `.agent-specstar/**`

## Known Abstraction Opportunities

- A small set of shell functions for `wait_for_postgres`, `wait_for_lambda`, `invoke`, `assert_status`. Keep them inside the single script; do not extract a library.

## Allowed Abstraction Scope

Single-file shell helpers only. No sourced sub-scripts, no separate `lib/` directory.

## Required Tests

The script **is** the test. There are no separate tests for it. Validation is:

1. `./e2e-test.sh` from a clean checkout (no containers, no volume) exits 0 and prints clear per-step pass lines.
2. Forcing a failure (e.g., temporarily editing the script to assert `statusCode == 999`) makes the script exit non-zero, and `docker compose ps` afterward shows no running services (i.e., the trap fired).
3. Forcing a missing prerequisite (e.g., `PATH=/usr/bin:/bin ./e2e-test.sh` if `jq` lives elsewhere) makes the script exit non-zero before any container is started.

These are reviewer-level checks; no automated harness is added.

## Coverage Requirement

This step changes **zero Go executable lines**. Go-side coverage rules do not apply. The script itself is the validation gate for the entire feature; correctness is judged by the three checks under Required Tests.

## Failure Model

Fail-fast. The script must:

- Set `set -euo pipefail` on the very first executable line.
- Install `trap 'docker compose down --remove-orphans >/dev/null 2>&1 || true' EXIT` **before** running `docker compose up`. The `|| true` on the trap line is the **only** permitted suppression and is required so the trap itself never masks the script's real exit code; it is not a fallback for any business behavior.
- Verify `docker`, `docker compose`, `curl`, and `jq` are on `PATH` before any container action; abort with a clear message otherwise.
- Treat any timeout, transport failure, missing JSON field, or `statusCode` mismatch as a hard failure with a non-zero exit.

No business-logic fallbacks. No "leave environment up on failure" flag.

## Allowed Fallbacks

- The single `|| true` on the EXIT trap's `docker compose down` invocation, solely to keep the trap from overriding the script's real exit code. Documented inline in a comment.

## Acceptance Criteria

1. File exists at the repo root as `e2e-test.sh` with executable permissions.
2. First executable line is `set -euo pipefail`. The EXIT trap is installed before `docker compose up`.
3. Prerequisite check verifies `docker`, `docker compose`, `curl`, `jq` are available; on absence, prints a clear message and exits non-zero before starting any service.
4. Loads env values from `.env` if present, otherwise from defaults that match `.env.example`.
5. Runs `docker compose up --build -d`.
6. Waits for PostgreSQL via `docker compose exec -T postgres pg_isready ...`, polling every 1s for up to 30 attempts; failure aborts the script.
7. Waits for the Lambda RIE by POSTing the design's "unknown path" probe payload to `http://localhost:${LAMBDA_HOST_PORT:-9000}/2015-03-31/functions/function/invocations`, polling every 1s for up to 30 attempts; failure aborts the script.
8. Generates a fresh `order_id` per invocation using `uuidgen` if available, else `cat /proc/sys/kernel/random/uuid`, else `openssl rand -hex 16` reformatted as a UUID; if none works, the script exits non-zero before invoking.
9. Issues the deposit invocation with the design's payload (`client_id` `00000000-0000-0000-0000-000000000001`, `asset_id` `00000000-0000-0000-0000-000000000101`, `amount` `"10"`, `unit_price` `"100.00"`), parses the response with `jq -r '.statusCode'`, and asserts the value equals `201`. Prints `PASS` / `FAIL` for the step.
10. Issues the withdrawal invocation with the design's payload (same `client_id`, `instrument_id` `CDB-0001`, `desired_value` `"250.00"`, fresh `order_id`), parses the response with `jq -r '.statusCode'`, and asserts the value equals `200`. Prints `PASS` / `FAIL` for the step.
11. On success exits `0`; on any failure exits non-zero. In both cases the trap runs `docker compose down --remove-orphans` exactly once.
12. No file outside Allowed Write Paths is modified.

## Deferred Work

- none

## Escalation Conditions

- None of `uuidgen`, `/proc/sys/kernel/random/uuid`, `openssl` is available on the developer's machine. Escalate; do not invent a deterministic order_id (idempotency would mask real failures across runs).
- The deposit invocation legitimately returns `statusCode: 200` because of an idempotent replay across runs against a leftover volume. The script must still treat that as failure (the contract is `201` for fresh deposits); escalate to revisit the seed/volume reset story rather than relaxing the assertion.
