# Wallet Lambda — Local Development Environment

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose v2](https://docs.docker.com/compose/install/) (`docker compose` — not the legacy `docker-compose`)
- `curl`
- `jq`

## Getting Started

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

## Database Initialization Behavior

The PostgreSQL container uses the standard `/docker-entrypoint-initdb.d` mechanism:

- `migrations/001_initial_schema.sql` is mounted as `/docker-entrypoint-initdb.d/01-schema.sql`.
- `local-env/init/02-seed.sql` is mounted as `/docker-entrypoint-initdb.d/02-seed.sql`.

These scripts execute **only when the data directory is empty** (i.e., on first startup or after removing the volume). Re-running `docker compose up` against an existing volume does **not** re-execute the init scripts.

To reset the database to a clean state:

```bash
docker compose down -v
docker compose up --build
```

## Invoking the Lambda Locally

All invocations are HTTP POSTs to the Lambda RIE invoke endpoint:

```
POST http://localhost:9000/2015-03-31/functions/function/invocations
```

The request body is a JSON-encoded API Gateway HTTP API v2 event. The `body` field inside the envelope must be a **JSON-encoded string** (escaped), not a nested object.

### Deposit

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

### Withdrawal

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

## Seed Data Reference

### Clients

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

### Assets

| # | asset_id | product_type | instrument_id | asset_name |
|---|---|---|---|---|
| 1 | `00000000-0000-0000-0000-000000000101` | `CDB` | `CDB-0001` | `CDB Local Test` |
| 2 | `00000000-0000-0000-0000-000000000102` | `LF`  | `LF-0001`  | `LF Local Test` |
| 3 | `00000000-0000-0000-0000-000000000103` | `LCI` | `LCI-0001` | `LCI Local Test` |
| 4 | `00000000-0000-0000-0000-000000000104` | `LCA` | `LCA-0001` | `LCA Local Test` |
| 5 | `00000000-0000-0000-0000-000000000105` | `CRI` | `CRI-0001` | `CRI Local Test` |
| 6 | `00000000-0000-0000-0000-000000000106` | `CRA` | `CRA-0001` | `CRA Local Test` |
| 7 | `00000000-0000-0000-0000-000000000107` | `LFT` | `LFT-0001` | `LFT Local Test` |

## Running the End-to-End Test

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
