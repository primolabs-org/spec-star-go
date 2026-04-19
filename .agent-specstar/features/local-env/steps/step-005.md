# Step 005 - Repository README

## Metadata
- Feature: local-env
- Step: step-005
- Status: pending
- Depends On: step-004
- Last Updated: 2026-04-19

## Objective

Author the repository-root `README.md` so a developer with only Docker, Docker Compose v2, `curl`, and `jq` installed can, by following the README alone, bring up the local environment, invoke the deposit and withdrawal flows successfully via the Lambda RIE, and run `./e2e-test.sh` end-to-end. Content must match the contract in `.agent-specstar/features/local-env/design.md` § Functional Requirements (FR-35) and § Seed data exactly.

## In Scope

- New `README.md` at the repository root with the six required sections from the design.
- `curl` examples for `/deposits` and `/withdrawals` using the canonical API Gateway HTTP API v2 envelope from the design.
- The seeded clients table (10 rows) and seeded assets table (7 rows) reproduced verbatim from the design's Seed data section.

## Out of Scope

- Architecture deep-dives, ADRs, contributor guidelines, license, or coverage badges.
- Documentation for production deployment, observability, or any out-of-scope item from the design.
- Any change to Go source, `go.mod`, `go.sum`, `migrations/`, the Dockerfile, Compose, SQL, or the e2e script.

## Required Reads

- `.agent-specstar/features/local-env/design.md` — sections: Functional Requirements (FR-35), Invocation payload shape, Environment variables, Seed data, E2E scenario values, Success Criteria.
- `docker-compose.yml`, `.env.example` — confirm the exact commands, env var names, and host ports the README must document.
- `e2e-test.sh` — confirms the exact invocation behavior the README describes.

## Allowed Write Paths

- `README.md`

## Forbidden Paths

- `cmd/**`
- `internal/**`
- `go.mod`, `go.sum`
- `migrations/**`
- `Dockerfile`, `.dockerignore`
- `docker-compose.yml`
- `.env`, `.env.example`
- `local-env/**`
- `e2e-test.sh`
- `.agent-specstar/**`

## Known Abstraction Opportunities

None. README is a flat document.

## Allowed Abstraction Scope

None.

## Required Tests

No automated tests. Validation is by reviewer walk-through:

1. A developer who has never seen the repo can copy each command from the README into a terminal in order and reach a successful deposit and withdrawal.
2. The seed tables in the README match the values produced by `local-env/init/02-seed.sql` and the design (cross-checked by inspection).
3. The `curl` examples match the invocation envelope and JSON body fields used by `e2e-test.sh`.

## Coverage Requirement

This step changes **zero Go executable lines**. Go-side coverage rules do not apply. The README is documentation; correctness is judged by the reviewer walk-through above.

## Failure Model

Fail-fast (documentation-level). Any drift between the README and the underlying design / Compose / seed / script is a defect to be fixed in this step, not deferred.

## Allowed Fallbacks

- none

## Acceptance Criteria

1. `README.md` exists at the repo root and contains the six sections required by FR-35: Prerequisites, Getting Started, Database Initialization Behavior, Invoking the Lambda Locally, Seed Data Reference, Running the End-to-End Test.
2. Prerequisites lists Docker, Docker Compose v2, `curl`, and `jq`.
3. Getting Started shows `cp .env.example .env`, `docker compose up --build`, basic `docker compose ps` / `docker compose logs` usage, and both `docker compose down` and `docker compose down -v` with their distinct effects.
4. Database Initialization Behavior explains that `/docker-entrypoint-initdb.d` scripts run only on a fresh data directory, that `migrations/001_initial_schema.sql` is mounted as `01-schema.sql` and `local-env/init/02-seed.sql` as `02-seed.sql`, and that `docker compose down -v` is the reset path.
5. Invoking the Lambda Locally provides a complete, copy-pasteable `curl` example for `/deposits` and one for `/withdrawals`, each posting the design's API Gateway HTTP API v2 envelope to `http://localhost:9000/2015-03-31/functions/function/invocations` with the seeded `client_id` `00000000-0000-0000-0000-000000000001`, the CDB asset, the values from § E2E scenario values, and a placeholder `order_id` the reader replaces with a fresh UUID. Each example shows piping through `jq` to inspect the embedded `statusCode` and `body`.
6. Seed Data Reference reproduces the 10-client table and the 7-asset table verbatim from the design (same columns, same UUIDs, same order).
7. Running the End-to-End Test documents `./e2e-test.sh`, what it asserts (deposit `statusCode` `201`, withdrawal `statusCode` `200`), that it always tears the environment down via a trap, and the exit-code contract.
8. No file outside Allowed Write Paths is modified.

## Deferred Work

- none

## Escalation Conditions

- The design's seed values, scenario values, or invocation envelope are revised after this README is written. The README must be updated in the same change set; escalate if those revisions land in a separate step.
