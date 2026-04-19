# Local Development Environment

## Metadata
- Feature: local-env
- Status: ready-for-implementation
- Owner: SpecStar
- Last Updated: 2026-04-19
- Source Request: Provide a Docker Compose-based local environment that runs the existing Go AWS Lambda HTTP API and PostgreSQL with a single command, invokable through the AWS Lambda Runtime Interface Emulator (RIE), and an end-to-end test script that exercises deposit and withdrawal flows.

## Problem Statement

Developers currently have no documented, reproducible way to run the wallet Lambda and its PostgreSQL backend locally for functional testing. There is no local invocation path that exercises the **same** Lambda HTTP handler used in deployment, and no automated end-to-end check that the deposit and withdrawal HTTP flows work against a real database. Onboarding, manual validation, and pre-deployment sanity checks all require ad hoc setup that is not captured in the repository.

## Goal

Deliver a self-contained local environment, started with a single `docker compose up --build` command, that runs:

1. A PostgreSQL container pre-initialized with the wallet schema and deterministic seed data.
2. The wallet Lambda container built from the existing `cmd/http-lambda/main.go` entrypoint, exposed via the AWS Lambda Runtime Interface Emulator (RIE) so deposit and withdrawal flows can be invoked from `curl` or Postman.
3. An `e2e-test.sh` script at the repo root that brings the environment up, invokes deposit and withdrawal end-to-end through the local Lambda invoke endpoint, asserts success, and tears the environment down.
4. A `README.md` documenting prerequisites, usage, seed data, local Lambda invocation examples, and the end-to-end test workflow.

## Functional Requirements

### Single-command bring-up

- FR-1: From the repository root, `docker compose up --build` starts both the PostgreSQL service and the wallet Lambda service, with the Lambda waiting for PostgreSQL to be healthy before starting.
- FR-2: `docker compose down` stops the environment cleanly. `docker compose down -v` additionally removes the PostgreSQL named volume, restoring a clean database state.
- FR-3: `docker compose up --build` rebuilds the Lambda image after Go source changes without manual cache invalidation.
- FR-4: The environment runs successfully on a clean machine that has only Docker and Docker Compose v2 installed (no Go toolchain, no LocalStack, no SAM, no extra local-only application binary required).

### Lambda container

- FR-5: The Lambda runs from the existing entrypoint `cmd/http-lambda/main.go`. No new `cmd/http-local` or non-Lambda HTTP server is introduced.
- FR-6: The Lambda image is built with a multi-stage Dockerfile per the contract in [Technical Approach — Lambda image](#lambda-image).
- FR-7: The Lambda container is invokable locally via the AWS Lambda Runtime Interface Emulator endpoint `POST /2015-03-31/functions/function/invocations`, reachable from the host on port `9000` (mapped to container port `8080`).
- FR-8: The Lambda container reads its database configuration from environment variables, at minimum `DATABASE_URL`, matching the existing `platform.LoadDatabaseConfig` contract.
- FR-9: The Lambda container connects to the PostgreSQL service over the Compose network using the service hostname `postgres` (not `localhost`).

### PostgreSQL container

- FR-10: Uses the official `postgres:16-alpine` image (pinned major version).
- FR-11: Database name, user, and password are configured via environment variables sourced from `.env` (with safe local defaults documented in `.env.example`).
- FR-12: PostgreSQL exposes its default port `5432` to the host so developers can inspect the database with external tools.
- FR-13: A named Docker volume (`postgres-data`) backs the PostgreSQL data directory so data survives container restarts (but is reset by `docker compose down -v`).
- FR-14: A `pg_isready`-based healthcheck is defined so dependent services can wait for PostgreSQL readiness via `depends_on.condition: service_healthy`.
- FR-15: SQL initialization scripts are mounted read-only into `/docker-entrypoint-initdb.d` and execute automatically on first startup (i.e., when the data volume is empty).

### Database initialization and seed data

- FR-16: The schema is initialized by mounting the existing `migrations/001_initial_schema.sql` verbatim into `/docker-entrypoint-initdb.d/01-schema.sql`. There is no separately maintained schema copy; the migration file is the single source of truth.
- FR-17: Seed data inserts exactly 10 clients with stable, deterministic UUIDs (see [Seed Data](#seed-data)).
- FR-18: Seed data inserts exactly one asset for each supported product type: `CDB`, `LF`, `LCI`, `LCA`, `CRI`, `CRA`, `LFT` (7 assets total), each with a stable, deterministic UUID and a stable `instrument_id`.
- FR-19: Seed data is sufficient for deposit and withdrawal flows to be exercised against the seeded clients and instrument IDs without requiring manual `INSERT` statements.
- FR-20: Re-running `docker compose up` against an existing data volume does not re-execute the init scripts (standard PostgreSQL image behavior). The README documents `docker compose down -v` as the reset path.

### Local Lambda invocation contract

- FR-21: Requests are sent to the Lambda RIE invoke endpoint `POST http://localhost:9000/2015-03-31/functions/function/invocations`. There is no local web server fronting the Lambda.
- FR-22: Request payloads are JSON-encoded API Gateway HTTP API v2 events (`events.APIGatewayV2HTTPRequest` shape) per [Invocation Payload Shape](#invocation-payload-shape).
- FR-23: The path values used by clients, the README, and `e2e-test.sh` MUST match the routes registered in `cmd/http-lambda/main.go`: `/deposits` and `/withdrawals`.

### End-to-end test script (`e2e-test.sh`)

- FR-24: Lives at the repository root and is executable.
- FR-25: Brings the environment up with `docker compose up --build -d`.
- FR-26: Waits for PostgreSQL readiness using `pg_isready` via `docker compose exec`.
- FR-27: Waits until the Lambda RIE invoke endpoint is reachable and ready to accept requests before issuing test invocations.
- FR-28: Invokes a deposit by `POST`-ing an API Gateway HTTP API v2 event for path `/deposits` to the local Lambda invoke endpoint, using a seeded `client_id`, a seeded asset's `asset_id` (UUID), a fresh `order_id`, and the deposit values defined in [E2E Scenario Values](#e2e-scenario-values).
- FR-29: Asserts the deposit invocation returns a Lambda response payload whose embedded `statusCode` is `201`.
- FR-30: Invokes a withdrawal similarly against `/withdrawals` for the same client and asset's `instrument_id`, with a fresh `order_id` and the withdrawal value defined in [E2E Scenario Values](#e2e-scenario-values).
- FR-31: Asserts the withdrawal returns a Lambda response payload whose embedded `statusCode` is `200`.
- FR-32: Prints a clear pass/fail line per step.
- FR-33: Exits with a non-zero status on any failure (readiness wait timeout, invocation transport error, status mismatch, or assertion failure).
- FR-34: Tears the environment down with `docker compose down` unconditionally on exit, via a shell `trap`. There is no flag to leave the environment up.

### README documentation

- FR-35: A repo-root `README.md` includes the following sections:
  - Prerequisites: Docker, Docker Compose v2, `curl`, `jq`.
  - Getting started: `docker compose up --build`, how to inspect containers and logs, and `docker compose down` / `down -v`.
  - Database initialization behavior: explanation of `/docker-entrypoint-initdb.d`, that init scripts run only on a fresh data directory, and that `docker compose down -v` resets the database.
  - Invoking the Lambda locally: complete `curl` examples for deposit and withdrawal posting an API Gateway HTTP API v2 event payload to `http://localhost:9000/2015-03-31/functions/function/invocations`, using seeded IDs.
  - Seed data reference: the 10 seeded `client_id` UUIDs and the 7 seeded asset entries (`product_type`, `asset_id`, `instrument_id`).
  - Running the end-to-end test: `./e2e-test.sh` and what it validates.

### Environment configuration

- FR-36: A `.env.example` file at the repository root documents all required local environment variables with safe local default values (see [Environment Variables](#environment-variables)).
- FR-37: `.env` itself is not committed.

## Non-Functional Requirements

- NFR-1: Lightweight environment with fast incremental rebuilds. Dependency download is cached in its own Docker layer so source-only changes do not re-trigger `go mod download`.
- NFR-2: The final Lambda image is as small as reasonably possible. Only the compiled `bootstrap` binary is present in the runtime stage on top of the AWS-provided base image.
- NFR-3: No LocalStack, no SAM CLI, and no parallel local-only application entrypoint are introduced.
- NFR-4: No new Go dependencies. `go.mod` and `go.sum` are not modified by this feature.
- NFR-5: Seed data is fully deterministic across runs (stable UUIDs, stable instrument IDs, fixed dates) so test scripts and documentation can hard-code expected identifiers.
- NFR-6: The `e2e-test.sh` script runs to completion within a developer-acceptable time on a clean machine (target: a single end-to-end pass in under a minute on a typical developer laptop, dominated by image build time on first run).

## Scope

### In Scope

- A multi-stage `Dockerfile` at the repository root building the wallet Lambda container from `cmd/http-lambda/main.go`.
- A `docker-compose.yml` at the repository root wiring the PostgreSQL and Lambda services, healthchecks, named volume, environment variables, and exposed ports.
- A `.env.example` at the repository root.
- A seed file at `local-env/init/02-seed.sql`. The schema script is the existing `migrations/001_initial_schema.sql`, mounted directly via Compose volume.
- An `e2e-test.sh` at the repository root.
- A repo-root `README.md`.

### Out of Scope

- Production deployment of the Lambda or its infrastructure.
- Kubernetes, Helm, ECS, or any non-Compose orchestration.
- CI/CD pipelines or GitHub Actions workflows.
- Reverse proxies, ingress, or TLS termination in Compose.
- Observability stacks (Grafana, Tempo, Prometheus, Datadog, Jaeger, OpenSearch).
- LocalStack or SAM CLI integration.
- Automatic re-running of init scripts against an existing data volume.
- **Any modification to Go source files** under `cmd/`, `internal/`, `go.mod`, or `go.sum`. The route paths registered in `cmd/http-lambda/main.go` are treated as fixed.
- Authorization, authentication, or secrets management beyond plain environment variables for local use.
- Schema migration tooling or runtime migration execution.

## Constraints and Assumptions

- The service is a Go AWS Lambda HTTP API targeting Go 1.25.0 with module path `github.com/primolabs-org/spec-star-go`.
- The Lambda entrypoint at `cmd/http-lambda/main.go` already routes API Gateway HTTP API v2 events by `requestContext.http.path` to deposit and withdrawal handlers using exactly the paths `/deposits` and `/withdrawals`. **This feature uses these exact paths everywhere and does not modify the handler.**
- Database configuration is loaded by `platform.LoadDatabaseConfig`, which **requires** `DATABASE_URL`. No new configuration mechanism is introduced.
- The schema source of truth is `migrations/001_initial_schema.sql`. The local environment mounts this file verbatim; there is no second copy to keep in sync.
- PostgreSQL is targeted to be compatible with AWS Aurora PostgreSQL semantics (no Aurora-only features in seed scripts).
- Docker Compose v2 syntax is used (no `version:` top-level key required, `depends_on` supports `condition: service_healthy`).
- All required Go dependencies are already in `go.mod`. **No new Go dependencies are introduced by this feature.**
- The runtime base image `public.ecr.aws/lambda/provided:al2023` bundles the Lambda Runtime Interface Emulator and exposes the RIE invoke endpoint on container port `8080` when run locally without a sandbox.
- Developers have outbound network access to pull the chosen `postgres` and AWS Lambda base images.
- Developers have `curl` and `jq` installed locally for the README examples and `e2e-test.sh`.

## Existing Context

- `cmd/http-lambda/main.go` — single Lambda entrypoint, dispatching `/deposits` and `/withdrawals`. Reused as-is.
- `internal/platform/database.go` — `LoadDatabaseConfig` requires `DATABASE_URL`. The local environment must inject this for the Lambda container.
- `internal/application/deposit_service.go` — `DepositRequest` JSON fields: `client_id`, `asset_id` (UUID), `order_id`, `amount`, `unit_price`. Success response is `201 Created`.
- `internal/application/withdraw_service.go` — `WithdrawRequest` JSON fields: `client_id`, `instrument_id`, `order_id`, `desired_value`. Success response is `200 OK`.
- `migrations/001_initial_schema.sql` — schema source of truth for `clients`, `assets`, `positions`, and `processed_commands`, including the `product_type` `CHECK` constraint restricting assets to `CDB`, `LF`, `LCI`, `LCA`, `CRI`, `CRA`, `LFT`. Required asset NOT NULL columns (no defaults): `asset_id`, `instrument_id`, `product_type`, `emission_entity_id`, `asset_name`, `issuance_date`, `maturity_date`.
- `internal/domain/`, `internal/application/`, `internal/adapters/`, `internal/ports/` — application, domain, and adapter layers consumed by the Lambda. Untouched by this feature.
- `go.mod` — already provides every dependency needed. No additions.

## Technical Approach

### File and directory layout

All artifacts produced by this feature live at well-known paths:

| Path | Purpose |
|---|---|
| `Dockerfile` | Multi-stage build for the wallet Lambda container |
| `.dockerignore` | Excludes VCS, IDE, docs, and the local-env folder from Docker build context |
| `docker-compose.yml` | Compose definition for `postgres` and `lambda` services |
| `.env.example` | Documented defaults for local environment variables |
| `local-env/init/02-seed.sql` | Deterministic client and asset seed data |
| `e2e-test.sh` | End-to-end validation script |
| `README.md` | Developer documentation |

The schema script is **not** copied; `migrations/001_initial_schema.sql` is mounted directly by the `postgres` service into `/docker-entrypoint-initdb.d/01-schema.sql`.

### Lambda image

Multi-stage `Dockerfile`:

- **Stage 1 (`build`)** — `golang:1.25-bookworm` (or pinned equivalent that satisfies `go 1.25.0` in `go.mod`):
  1. `WORKDIR /src`
  2. `COPY go.mod go.sum ./`
  3. `RUN go mod download` (cached layer independent of source changes)
  4. `COPY . .`
  5. `RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bootstrap ./cmd/http-lambda`
- **Stage 2 (runtime)** — `public.ecr.aws/lambda/provided:al2023`:
  1. `COPY --from=build /out/bootstrap ${LAMBDA_RUNTIME_DIR}/bootstrap` (the `provided.al2023` image expects the binary at `/var/runtime/bootstrap`, which is what `${LAMBDA_RUNTIME_DIR}` resolves to)
  2. `CMD [ "function.handler" ]` — the handler name is required by the image entrypoint contract but is not consulted by `provided` runtimes; any non-empty string is acceptable.

The runtime image's default entrypoint is the AWS Lambda Runtime Interface Emulator when the container is run locally without an `AWS_LAMBDA_RUNTIME_API` environment variable. The emulator listens on container port `8080`.

### Compose topology

Two services, single user-defined network (Compose default):

- **`postgres`** — `postgres:16-alpine`
  - Env: `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` from `.env`.
  - Ports: `${POSTGRES_HOST_PORT:-5432}:5432`.
  - Volumes:
    - `postgres-data:/var/lib/postgresql/data` (named volume).
    - `./migrations/001_initial_schema.sql:/docker-entrypoint-initdb.d/01-schema.sql:ro`.
    - `./local-env/init/02-seed.sql:/docker-entrypoint-initdb.d/02-seed.sql:ro`.
  - Healthcheck: `pg_isready -U ${POSTGRES_USER} -d ${POSTGRES_DB}`, `interval: 5s`, `timeout: 5s`, `retries: 10`.
- **`lambda`** — `build: .` (uses the repo-root `Dockerfile`)
  - Env: `DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable`.
  - Ports: `${LAMBDA_HOST_PORT:-9000}:8080`.
  - `depends_on: { postgres: { condition: service_healthy } }`.

The numeric prefixes `01-` and `02-` on the init scripts pin execution order: schema first, seed second.

### Invocation payload shape

All local invocations POST a single JSON object matching the minimal valid `events.APIGatewayV2HTTPRequest` envelope. Canonical example used in the README and `e2e-test.sh`:

```json
{
  "version": "2.0",
  "routeKey": "POST /deposits",
  "rawPath": "/deposits",
  "requestContext": {
    "http": {
      "method": "POST",
      "path": "/deposits",
      "protocol": "HTTP/1.1",
      "sourceIp": "127.0.0.1",
      "userAgent": "curl"
    }
  },
  "headers": { "content-type": "application/json" },
  "body": "{\"client_id\":\"...\",\"asset_id\":\"...\",\"order_id\":\"...\",\"amount\":\"10\",\"unit_price\":\"100.00\"}",
  "isBase64Encoded": false
}
```

The withdrawal payload differs only in `routeKey`, `rawPath`, `requestContext.http.path`, and the `body` JSON (which uses `instrument_id` and `desired_value` instead of `asset_id`, `amount`, and `unit_price`).

### Environment variables

`.env.example` keys and defaults:

| Key | Default | Used by |
|---|---|---|
| `POSTGRES_DB` | `wallet` | `postgres`, composed into `DATABASE_URL` |
| `POSTGRES_USER` | `wallet` | `postgres`, composed into `DATABASE_URL` |
| `POSTGRES_PASSWORD` | `wallet` | `postgres`, composed into `DATABASE_URL` |
| `POSTGRES_HOST_PORT` | `5432` | host port for `postgres` |
| `LAMBDA_HOST_PORT` | `9000` | host port for `lambda` RIE |

The composed internal `DATABASE_URL` injected into the `lambda` service is:

```
postgres://wallet:wallet@postgres:5432/wallet?sslmode=disable
```

`DATABASE_URL` is **not** declared in `.env.example`; it is composed by `docker-compose.yml` from the four `POSTGRES_*` keys so the developer cannot accidentally desync credentials between the two services.

### Seed data

10 clients, deterministic UUIDs `00000000-0000-0000-0000-00000000000X` (X = 1..10), `external_id` of the form `CLIENT-001`..`CLIENT-010`:

| # | client_id | external_id |
|---|---|---|
| 1 | `00000000-0000-0000-0000-000000000001` | `CLIENT-001` |
| 2 | `00000000-0000-0000-0000-000000000002` | `CLIENT-002` |
| 3 | `00000000-0000-0000-0000-000000000003` | `CLIENT-003` |
| 4 | `00000000-0000-0000-0000-000000000004` | `CLIENT-004` |
| 5 | `00000000-0000-0000-0000-000000000005` | `CLIENT-005` |
| 6 | `00000000-0000-0000-0000-000000000006` | `CLIENT-006` |
| 7 | `00000000-0000-0000-0000-000000000007` | `CLIENT-007` |
| 8 | `00000000-0000-0000-0000-000000000008` | `CLIENT-008` |
| 9 | `00000000-0000-0000-0000-000000000009` | `CLIENT-009` |
| 10 | `00000000-0000-0000-0000-000000000010` | `CLIENT-010` |

7 assets, deterministic UUIDs `00000000-0000-0000-0000-00000000010X` (X = 1..7), one per `product_type`. All assets share `emission_entity_id = 'EMISSOR-LOCAL'`, `issuance_date = '2026-01-01'`, `maturity_date = '2030-01-01'`. Asset names follow `<product_type> Local Test`:

| # | asset_id | product_type | instrument_id | asset_name |
|---|---|---|---|---|
| 1 | `00000000-0000-0000-0000-000000000101` | `CDB` | `CDB-0001` | `CDB Local Test` |
| 2 | `00000000-0000-0000-0000-000000000102` | `LF`  | `LF-0001`  | `LF Local Test` |
| 3 | `00000000-0000-0000-0000-000000000103` | `LCI` | `LCI-0001` | `LCI Local Test` |
| 4 | `00000000-0000-0000-0000-000000000104` | `LCA` | `LCA-0001` | `LCA Local Test` |
| 5 | `00000000-0000-0000-0000-000000000105` | `CRI` | `CRI-0001` | `CRI Local Test` |
| 6 | `00000000-0000-0000-0000-000000000106` | `CRA` | `CRA-0001` | `CRA Local Test` |
| 7 | `00000000-0000-0000-0000-000000000107` | `LFT` | `LFT-0001` | `LFT Local Test` |

Optional columns (`offer_id`, `issuer_document_id`, `market_code`) are left `NULL`. `created_at` defaults to `now()` per the schema.

The seed script uses `INSERT ... ON CONFLICT DO NOTHING` so that re-applying it manually (e.g., via `psql -f`) is safe even though normal init runs only once.

### E2E scenario values

The e2e script uses:

- **Client**: `00000000-0000-0000-0000-000000000001` (CLIENT-001).
- **Asset**: `00000000-0000-0000-0000-000000000101` (CDB), `instrument_id = CDB-0001`.
- **Deposit**: `amount = "10"`, `unit_price = "100.00"` (total_value = `1000.00`).
- **Withdrawal**: `desired_value = "250.00"` (strictly less than the deposited total).
- **`order_id`**: a fresh UUID generated per invocation (`uuidgen` if available, otherwise `cat /proc/sys/kernel/random/uuid`, otherwise an `openssl rand -hex 16`-derived value). The script picks the first available method and fails fast if none works.

### E2E readiness waits

- **PostgreSQL**: poll `docker compose exec -T postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"` every 1s for up to 30 attempts. Compose's `depends_on: service_healthy` already gates the Lambda container start; this check is a script-level safety net plus a clear failure signal.
- **Lambda RIE**: poll the invoke endpoint with a benign probe payload (an event for an unknown path, expected to return a Lambda response with `statusCode: 404`) every 1s for up to 30 attempts. Any 200-class HTTP response from the RIE — regardless of the embedded Lambda `statusCode` — proves the container is accepting invocations. Failure to get any 200-class HTTP response within the budget aborts the script.

### E2E status assertion

The Lambda RIE returns the function's return value as a JSON document. For an `events.APIGatewayV2HTTPResponse`, that document has a top-level `statusCode` integer field. The script extracts it with `jq -r '.statusCode'` and compares against the expected value (`201` for deposit, `200` for withdrawal). Any mismatch, missing field, or non-2xx HTTP transport status from the RIE itself fails the script.

`jq` is the chosen JSON tool. It is listed as a prerequisite in the README and in the script's prerequisite check (the script verifies `jq` is on `PATH` before doing anything else).

## Affected Components

| Component | Change |
|---|---|
| `Dockerfile` (new) | Multi-stage Lambda image |
| `.dockerignore` (new) | Trim build context |
| `docker-compose.yml` (new) | Compose topology |
| `.env.example` (new) | Documented env var defaults |
| `local-env/init/02-seed.sql` (new) | Seed data |
| `e2e-test.sh` (new) | End-to-end validation script |
| `README.md` (new) | Developer documentation |
| `migrations/001_initial_schema.sql` | **Read-only mount target. Not modified.** |
| `cmd/http-lambda/main.go` | **Not modified.** |
| `internal/**`, `go.mod`, `go.sum` | **Not modified.** |

**No Go source file is created or modified by this feature.**

## Contracts and Data Shape Impact

- No changes to any Go-side request/response contract.
- The local environment uses the existing `DepositRequest`/`WithdrawRequest` JSON shapes (see [Existing Context](#existing-context)).
- The invocation envelope is the standard `events.APIGatewayV2HTTPRequest` shape (see [Invocation Payload Shape](#invocation-payload-shape)).

## State / Persistence Impact

- New named Docker volume `postgres-data` for the local PostgreSQL data directory.
- Initial schema and seed data populated automatically on first volume creation.
- No production state, no migration tooling, no data lifecycle management beyond `docker compose down -v`.

## Failure Model

Default: **fail-fast**. No silent fallbacks anywhere.

| Condition | Behavior |
|---|---|
| `Dockerfile` build error | `docker compose up --build` fails; no container starts. |
| Missing `.env` and missing default | Compose reports the missing variable and exits non-zero. |
| Schema or seed SQL error | PostgreSQL container exits during init; `lambda` never becomes healthy because `postgres` never reports healthy. |
| `DATABASE_URL` missing or malformed | `cmd/http-lambda/main.go` calls `log.Fatalf` via `LoadDatabaseConfig`; container exits. |
| `pg_isready` does not succeed within budget | `e2e-test.sh` exits non-zero with a clear message; trap tears the environment down. |
| RIE invoke endpoint not reachable within budget | `e2e-test.sh` exits non-zero; trap tears the environment down. |
| `jq` missing | `e2e-test.sh` exits non-zero before invoking anything. |
| Embedded `statusCode` mismatch | `e2e-test.sh` exits non-zero; trap tears the environment down. |
| Any unhandled error in the script | `set -euo pipefail` propagates; trap tears the environment down. |

There are no allowed fallbacks. There is no flag to keep the environment up on failure.

## Testing and Validation Strategy

This feature **changes zero Go executable lines**. The repository's Go-coverage rule does not apply to this feature because there is no Go code to cover.

The validation harness for this feature is `e2e-test.sh` itself. It is the only automated gate, and it covers:

- Image build success.
- Compose bring-up to a healthy state.
- Schema and seed initialization (proven implicitly by the deposit insert succeeding against a foreign-keyed seeded client and asset).
- End-to-end deposit happy path (`201`).
- End-to-end withdrawal happy path (`200`).
- Clean teardown on both success and failure.

Each step file restates this Go-coverage exemption in its Coverage Requirement section.

## Execution Notes

- `cmd/http-lambda/main.go` is the **only** Lambda entrypoint and is not modified. The route paths it registers (`/deposits`, `/withdrawals`) are authoritative for everything in this feature.
- The schema is mounted, not copied. There is no schema-drift risk because there is no second copy.
- The Lambda runtime base image `public.ecr.aws/lambda/provided:al2023` ships with the RIE; no extra binary is downloaded or installed.
- Compose service names `postgres` and `lambda` are part of the contract: the internal `DATABASE_URL` resolves `postgres` via the Compose-managed DNS, and `e2e-test.sh` uses `docker compose exec postgres ...` for readiness checks.
- Host-side ports: `5432` for PostgreSQL, `9000` for the Lambda RIE. The container-side Lambda port is `8080`.
- The 7 product types are pinned by the schema's `CHECK` constraint; the seed exercises each one to surface any future schema-side typo.
- `e2e-test.sh` MUST set `set -euo pipefail` at the top and install the `trap '... ' EXIT` before starting the environment so cleanup always runs.

## Open Questions

None. All design questions have been resolved.

## Success Criteria

- SC-1: On a clean machine with Docker and Docker Compose v2 installed, `docker compose up --build` from the repository root brings up PostgreSQL (initialized with the wallet schema and seed data) and the wallet Lambda container, with the Lambda reachable on host port `9000` via the AWS Lambda RIE invoke endpoint.
- SC-2: A developer can issue the README's `curl` example for `/deposits` and receive a Lambda response payload with `"statusCode": 201` and a valid created-position body.
- SC-3: A developer can issue the README's `curl` example for `/withdrawals` and receive a Lambda response payload with `"statusCode": 200` and an affected-lots body.
- SC-4: `./e2e-test.sh` executed from the repository root brings the environment up, executes the deposit and withdrawal scenarios end-to-end, prints clear per-step pass/fail output, exits zero on full success, exits non-zero on any failure, and tears the environment down in both cases via a trap.
- SC-5: `docker compose down` cleanly stops all services. `docker compose down -v` additionally removes the `postgres-data` volume so the next `docker compose up` re-runs the init scripts against a fresh data directory.
- SC-6: No new Go dependencies are added to `go.mod`. No LocalStack, SAM, or non-Lambda local web server is introduced. The Lambda runs from the unmodified `cmd/http-lambda/main.go`.
- SC-7: Seed data uses the deterministic UUIDs and instrument IDs documented in [Seed Data](#seed-data), and these values match the README and `e2e-test.sh` exactly.
- SC-8: No file under `cmd/`, `internal/`, `go.mod`, or `go.sum` is modified by this feature.
