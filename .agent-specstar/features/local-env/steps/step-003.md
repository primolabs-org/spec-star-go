# Step 003 - Docker Compose Topology and Environment Defaults

## Metadata
- Feature: local-env
- Step: step-003
- Status: pending
- Depends On: step-001, step-002
- Last Updated: 2026-04-19

## Objective

Author `docker-compose.yml` and `.env.example` at the repository root so that `docker compose up --build` brings up the `postgres` and `lambda` services per `.agent-specstar/features/local-env/design.md` § Compose topology, with the schema (`migrations/001_initial_schema.sql`) and seed (`local-env/init/02-seed.sql`) mounted in the correct order, the named volume `postgres-data` provisioned, healthchecks wired, and the Lambda reachable on host port `9000`.

## In Scope

- New `docker-compose.yml` at the repository root defining services `postgres` and `lambda`, the named volume `postgres-data`, and the read-only init mounts.
- New `.env.example` at the repository root with the exact keys and defaults from the design's Environment Variables table.

## Out of Scope

- The Dockerfile (step 001).
- The seed SQL (step 002).
- The e2e script (step 004).
- The README (step 005).
- Any change to Go source, `go.mod`, `go.sum`, or `migrations/`.

## Required Reads

- `.agent-specstar/features/local-env/design.md` — sections: Technical Approach → Compose topology, Environment variables, Failure Model.
- `Dockerfile` (produced by step 001) — confirms `build: .` is sufficient.
- `local-env/init/02-seed.sql` (produced by step 002) — confirms the mount source path.
- `migrations/001_initial_schema.sql` — confirms the schema mount source path.
- `internal/platform/database.go` — confirms `DATABASE_URL` is the only required env var.

## Allowed Write Paths

- `docker-compose.yml`
- `.env.example`

## Forbidden Paths

- `cmd/**`
- `internal/**`
- `go.mod`, `go.sum`
- `migrations/**`
- `Dockerfile`, `.dockerignore`
- `local-env/**`
- `e2e-test.sh`
- `README.md`
- `.env` (must remain uncommitted; only `.env.example` is created)
- `.agent-specstar/**`

## Known Abstraction Opportunities

None. A single Compose file with two services is the minimum viable shape and matches the design exactly.

## Allowed Abstraction Scope

None. Do not introduce profiles, override files, build args, or extra services.

## Required Tests

This step changes no Go code. It is validated by Compose-level checks executed manually or as part of step 004:

1. `docker compose config` succeeds and reports both services with the documented mounts, env vars, ports, and healthcheck.
2. `docker compose up --build -d` brings both services up; `docker compose ps` reports `postgres` as `healthy` and `lambda` as `running` within the healthcheck budget.
3. `curl -fsS -X POST -H 'Content-Type: application/json' -d '{"version":"2.0","rawPath":"/unknown","requestContext":{"http":{"method":"POST","path":"/unknown"}},"isBase64Encoded":false}' http://localhost:9000/2015-03-31/functions/function/invocations` returns HTTP 200 from the RIE.
4. `docker compose down` and `docker compose down -v` both succeed; the latter removes the `postgres-data` volume.

## Coverage Requirement

This step changes **zero Go executable lines**. Go-side coverage rules do not apply. The Compose definition is validated by the bring-up checks listed under Required Tests and re-validated end-to-end by step 004.

## Failure Model

Fail-fast. Compose must fail loudly on missing variables, missing files, or unhealthy services. Do not use `restart: always`, do not weaken the `depends_on` condition below `service_healthy`, and do not introduce silent retry loops.

## Allowed Fallbacks

- none

## Acceptance Criteria

1. `docker-compose.yml` defines exactly two services with the names `postgres` and `lambda`. No `version:` top-level key.
2. `postgres` service:
   - Image `postgres:16-alpine`.
   - Env `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` sourced from the developer's `.env` (with the `.env.example` defaults).
   - Port mapping `${POSTGRES_HOST_PORT:-5432}:5432`.
   - Volumes: named `postgres-data` mounted at `/var/lib/postgresql/data`; `./migrations/001_initial_schema.sql` mounted read-only at `/docker-entrypoint-initdb.d/01-schema.sql`; `./local-env/init/02-seed.sql` mounted read-only at `/docker-entrypoint-initdb.d/02-seed.sql`.
   - Healthcheck running `pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"` with `interval: 5s`, `timeout: 5s`, `retries: 10`.
3. `lambda` service:
   - `build: .` (uses the repo-root `Dockerfile`).
   - Env `DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable` composed inline by Compose. `DATABASE_URL` is **not** declared in `.env.example`.
   - Port mapping `${LAMBDA_HOST_PORT:-9000}:8080`.
   - `depends_on: { postgres: { condition: service_healthy } }`.
4. Top-level `volumes:` declares `postgres-data:` with default driver.
5. `.env.example` contains exactly the five keys from the design (`POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_HOST_PORT`, `LAMBDA_HOST_PORT`) with the documented defaults, and no other keys (in particular, no `DATABASE_URL`).
6. The Required Tests checks 1–4 pass on a clean machine.
7. No file outside Allowed Write Paths is modified.

## Deferred Work

- none

## Escalation Conditions

- The `pg_isready` binary is missing from `postgres:16-alpine` (it is not, but if a future image bump removes it, escalate before substituting a different healthcheck).
- The developer's host already has port `5432` or `9000` in use. Document the override via `.env`; do not change defaults.
