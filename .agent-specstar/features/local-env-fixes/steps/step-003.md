# Step 003 - Make Docker Compose PostgreSQL bootstrap fail-fast on missing credentials

## Metadata
- Feature: local-env-fixes
- Step: step-003
- Status: pending
- Depends On: step-002
- Last Updated: 2026-04-19

## Objective

Switch `docker-compose.yml` to required-variable interpolation for the PostgreSQL credential variables so `docker compose up` aborts with a clear error before any container starts when `POSTGRES_DB`, `POSTGRES_USER`, or `POSTGRES_PASSWORD` is unset or empty. Satisfies FR-10 through FR-14 of `design.md`.

## In Scope

- Edit `docker-compose.yml` to use `${VAR:?message}` interpolation for `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD`, both in the `postgres` service environment block and inside the `lambda` service's composed `DATABASE_URL`.
- Verify `.env.example` still ships non-empty defaults for all three variables. Update only if a value is missing or empty.
- Run a Compose smoke validation (see Required Tests).

## Out of Scope

- Editing the `Dockerfile`, `.golangci.yml`, `go.mod`, `go.sum`, or any Go source.
- Editing `.github/agents/AGENTS.md` (next step).
- Changing port ergonomics for `POSTGRES_HOST_PORT` and `LAMBDA_HOST_PORT`. Their `${VAR:-N}` defaults stay as-is.
- Introducing new environment variables.
- Changing volume mounts or healthcheck definitions.

## Required Reads

- `.agent-specstar/features/local-env-fixes/design.md` (sections "Docker Compose required variables", "Failure Model")
- `docker-compose.yml`
- `.env.example`
- `e2e-test.sh` (to confirm no implicit assumption breaks)
- `.github/agents/AGENTS.md` (section 13)
- `.github/instructions/fail-fast.instructions.md`
- `.github/instructions/cleanup.instructions.md`

## Allowed Write Paths

- `docker-compose.yml`
- `.env.example` (only if a required default is missing or empty)

## Forbidden Paths

- everything else

## Known Abstraction Opportunities

- none

## Allowed Abstraction Scope

None. The change is a literal interpolation form swap for three variables in two locations.

## Required Tests

- `docker compose config` succeeds with `.env` populated from `.env.example` and shows the three credential variables resolved to their non-empty values.
- `POSTGRES_PASSWORD= docker compose config` (or the equivalent on Windows: invoking compose with `POSTGRES_PASSWORD` explicitly set to an empty string) fails with the configured error message and a non-zero exit code, BEFORE any container is created.
- `docker compose up --build -d` succeeds with a populated `.env`.
- `docker compose ps` shows both services running and `postgres` healthy.
- `docker compose down` tears the stack down cleanly.
- `e2e-test.sh` runs end-to-end successfully with a populated `.env`. (Run only if Docker is available; otherwise report it as not run.)

If Docker is not available on the implementation host, the step MUST still merge the configuration change but MUST explicitly report which Docker validations were skipped and why.

## Coverage Requirement

Not applicable. No executable Go lines change.

## Failure Model

Fail-fast. Do not introduce `${VAR:-default}` defaults for the three credential variables — the whole point of the step is to refuse to start with empty credentials. Do not add a startup wrapper script that tries to recover from missing variables.

## Allowed Fallbacks

- none

## Acceptance Criteria

- `docker-compose.yml` uses `${POSTGRES_DB:?...}`, `${POSTGRES_USER:?...}`, and `${POSTGRES_PASSWORD:?...}` everywhere those variables are referenced (including inside the composed `DATABASE_URL` for the `lambda` service).
- The error messages embedded in the interpolation form clearly state what the operator must do (e.g., `"POSTGRES_PASSWORD must be set and non-empty"`).
- With a populated `.env`, the full local stack starts, `postgres` becomes healthy, the Lambda RIE accepts invocations, and `e2e-test.sh` passes.
- With `POSTGRES_PASSWORD` explicitly empty, `docker compose up` aborts before any container is created and the embedded error message is visible.
- `.env.example` still ships non-empty defaults for `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD`.
- Completion report lists the exact validation commands run and their outcomes; any skipped command is explicitly reported with the reason.

## Deferred Work

- `AGENTS.md` verification is deferred to step-004.

## Escalation Conditions

- Compose validation rejects the `${VAR:?message}` syntax (would indicate an unsupported Compose version).
- The Lambda container fails to connect to PostgreSQL after the change, which would indicate a credential desync between the two services.
- `e2e-test.sh` regresses in any way attributable to this change.
