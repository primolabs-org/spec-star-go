# Wallet Lambda

A Go microservice deployed on AWS Lambda that manages fixed-income wallet positions (deposits and withdrawals) backed by PostgreSQL.

## Overview

The service exposes two operations via API Gateway HTTP API v2:

- **POST /deposits** — Creates a new position (deposit lot) for a client and asset.
- **POST /withdrawals** — Reduces position amounts across one or more lots to satisfy a desired withdrawal value (FIFO-like, by available value).

Both operations are idempotent: repeating the same `order_id` replays the original response without side effects.

## Tech Stack

| Component | Technology |
|---|---|
| Language | Go 1.26 |
| Runtime | AWS Lambda (`provided:al2023`) |
| API Protocol | API Gateway HTTP API v2 events |
| Database | PostgreSQL 16 (`pgx/v5` driver, connection pool) |
| Decimal math | `shopspring/decimal` |
| Structured logging | `log/slog` (JSON to stdout) |

## Project Structure

```
cmd/
  http-lambda/          Lambda entry point — routes requests to handlers
internal/
  domain/               Value objects, entities, and domain errors
  application/          Use-case orchestration (DepositService, WithdrawService)
  ports/                Repository and UnitOfWork interfaces
  adapters/
    inbound/httphandler/  API Gateway v2 request/response mapping
    outbound/postgres/    pgx-based repository and transaction implementations
  platform/             Cross-cutting: database pool config, structured logging
migrations/             SQL schema definitions
local-env/              Seed data for local development
```

### Architecture

The codebase follows a hexagonal (ports-and-adapters) architecture:

- **Domain** — Pure business logic with no framework dependencies. Entities (`Client`, `Asset`, `Position`, `ProcessedCommand`), value objects (`ProductType`), and typed errors (`ValidationError`, `ErrNotFound`, `ErrConcurrencyConflict`, `ErrInsufficientPosition`).
- **Ports** — Interfaces that the domain and application layers depend on: `ClientRepository`, `AssetRepository`, `PositionRepository`, `ProcessedCommandRepository`, `UnitOfWork`.
- **Application** — Service layer that coordinates domain operations, idempotency checks, input validation, and transactional persistence.
- **Adapters (inbound)** — HTTP handlers that translate API Gateway v2 events into application-layer calls and back into HTTP responses.
- **Adapters (outbound)** — PostgreSQL implementations of port interfaces using `pgx/v5`, including a `TransactionRunner` that propagates transactions via context.
- **Platform** — Shared infrastructure: connection pool configuration (`DATABASE_URL`, tunable via environment variables), and a `LoggerFactory` that produces per-request loggers enriched with `request_id`, `trigger`, `operation`, and `cold_start`.

### Key Design Decisions

- **Idempotency via ProcessedCommand** — Every deposit/withdrawal records a `processed_commands` entry keyed by `(command_type, order_id)`. Duplicate requests return the stored response snapshot.
- **Optimistic concurrency** — Positions carry a `row_version` that is checked on update to detect concurrent modifications.
- **Lot-based withdrawals** — Withdrawals iterate eligible position lots in order, consuming available value until the desired amount is satisfied or an `ErrInsufficientPosition` is returned.
- **Supported product types** — `CDB`, `LF`, `LCI`, `LCA`, `CRI`, `CRA`, `LFT`.

### Database Schema

Four tables defined in `migrations/001_initial_schema.sql`:

| Table | Purpose |
|---|---|
| `clients` | Wallet clients with an `external_id` |
| `assets` | Fixed-income instruments (instrument, product type, issuer, dates) |
| `positions` | Deposit lots linking a client to an asset with amount, unit price, and collateral values |
| `processed_commands` | Idempotency log storing command type, order ID, and response snapshot (JSONB) |

### Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `DB_MAX_CONNECTIONS` | No | `3` | Maximum pool connections |
| `DB_MIN_CONNECTIONS` | No | `0` | Minimum pool connections |
| `DB_MAX_CONN_LIFETIME` | No | `5m` | Maximum connection lifetime |
| `DB_MAX_CONN_IDLE_TIME` | No | `30s` | Maximum idle time per connection |
| `DB_HEALTH_CHECK_PERIOD` | No | `15s` | Pool health check interval |

## Running Tests

```bash
go test ./...
```

## Local Development Environment

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose v2](https://docs.docker.com/compose/install/) (`docker compose` — not the legacy `docker-compose`)
- `curl`
- `jq`

### Getting Started

1. Copy the example environment file:

   ```bash
   cp .env.example .env
   ```

2. Build and start the environment:

   ```bash
   docker compose up --build
   ```

   This starts two services:
   - **postgres** — PostgreSQL 16, initialized with the wallet schema and seed data.
   - **lambda** — The wallet Lambda running behind the AWS Lambda Runtime Interface Emulator (RIE), reachable on host port `9000`.

3. Check service status and logs:

   ```bash
   docker compose ps
   docker compose logs
   docker compose logs lambda
   docker compose logs postgres
   ```

4. Stop the environment:

   ```bash
   # Stop services; database data is preserved in the named volume.
   docker compose down

   # Stop services AND remove the PostgreSQL data volume.
   # The next 'docker compose up' will re-run schema and seed scripts from scratch.
   docker compose down -v
   ```

### Database Initialization Behavior

The PostgreSQL container uses the standard `/docker-entrypoint-initdb.d` mechanism:

- `migrations/001_initial_schema.sql` is mounted as `/docker-entrypoint-initdb.d/01-schema.sql`.
- `local-env/init/02-seed.sql` is mounted as `/docker-entrypoint-initdb.d/02-seed.sql`.

These scripts execute **only when the data directory is empty** (i.e., on first startup or after removing the volume). Re-running `docker compose up` against an existing volume does **not** re-execute the init scripts.

To reset the database to a clean state:

```bash
docker compose down -v
docker compose up --build
```

### Invoking the Lambda Locally

All invocations are HTTP POSTs to the Lambda RIE invoke endpoint:

```
POST http://localhost:9000/2015-03-31/functions/function/invocations
```

The request body is a JSON-encoded API Gateway HTTP API v2 event. The `body` field inside the envelope must be a **JSON-encoded string** (escaped), not a nested object.

#### Deposit

```bash
curl -sS -X POST \
  http://localhost:9000/2015-03-31/functions/function/invocations \
  -d '{
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
    "body": "{\"client_id\":\"00000000-0000-0000-0000-000000000001\",\"asset_id\":\"00000000-0000-0000-0000-000000000101\",\"order_id\":\"<REPLACE-WITH-A-FRESH-UUID>\",\"amount\":\"10\",\"unit_price\":\"100.00\"}",
    "isBase64Encoded": false
  }' | jq .
```

Replace `<REPLACE-WITH-A-FRESH-UUID>` with a unique UUID (e.g., from `uuidgen`). Each `order_id` must be unique — the system rejects duplicate commands.

Expected: `"statusCode": 201` in the response.

#### Withdrawal

```bash
curl -sS -X POST \
  http://localhost:9000/2015-03-31/functions/function/invocations \
  -d '{
    "version": "2.0",
    "routeKey": "POST /withdrawals",
    "rawPath": "/withdrawals",
    "requestContext": {
      "http": {
        "method": "POST",
        "path": "/withdrawals",
        "protocol": "HTTP/1.1",
        "sourceIp": "127.0.0.1",
        "userAgent": "curl"
      }
    },
    "headers": { "content-type": "application/json" },
    "body": "{\"client_id\":\"00000000-0000-0000-0000-000000000001\",\"instrument_id\":\"CDB-0001\",\"order_id\":\"<REPLACE-WITH-A-FRESH-UUID>\",\"desired_value\":\"250.00\"}",
    "isBase64Encoded": false
  }' | jq .
```

Replace `<REPLACE-WITH-A-FRESH-UUID>` with a new unique UUID. A deposit for this client and asset must exist before withdrawing.

Expected: `"statusCode": 200` in the response.

### Seed Data Reference

#### Clients

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

#### Assets

| # | asset_id | product_type | instrument_id | asset_name |
|---|---|---|---|---|
| 1 | `00000000-0000-0000-0000-000000000101` | `CDB` | `CDB-0001` | `CDB Local Test` |
| 2 | `00000000-0000-0000-0000-000000000102` | `LF`  | `LF-0001`  | `LF Local Test` |
| 3 | `00000000-0000-0000-0000-000000000103` | `LCI` | `LCI-0001` | `LCI Local Test` |
| 4 | `00000000-0000-0000-0000-000000000104` | `LCA` | `LCA-0001` | `LCA Local Test` |
| 5 | `00000000-0000-0000-0000-000000000105` | `CRI` | `CRI-0001` | `CRI Local Test` |
| 6 | `00000000-0000-0000-0000-000000000106` | `CRA` | `CRA-0001` | `CRA Local Test` |
| 7 | `00000000-0000-0000-0000-000000000107` | `LFT` | `LFT-0001` | `LFT Local Test` |

### Running the End-to-End Test

```bash
./e2e-test.sh
```

The script:

1. Brings the environment up with `docker compose up --build -d`.
2. Waits for PostgreSQL readiness and Lambda RIE availability.
3. Invokes a deposit for client `00000000-0000-0000-0000-000000000001` against the CDB asset and asserts `statusCode` `201`.
4. Invokes a withdrawal for the same client and asset and asserts `statusCode` `200`.
5. Prints a clear PASS/FAIL line per step.
6. **Always** tears the environment down via a shell `trap`, regardless of success or failure.

Exit code: `0` on success, non-zero on any failure.
