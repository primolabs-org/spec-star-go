# Step 005 Review — README.md

## Outcome: APPROVED

All eight acceptance criteria pass. The README is accurate, complete, and consistent with every cross-referenced artifact.

---

## Per-Criterion Assessment

### CR-1: Six required sections — PASS

`README.md` contains exactly the six sections required by FR-35:
1. Prerequisites
2. Getting Started
3. Database Initialization Behavior
4. Invoking the Lambda Locally
5. Seed Data Reference
6. Running the End-to-End Test

No extra architecture, ADR, contributor, or license sections (correctly excluded per Out of Scope).

### CR-2: Prerequisites — PASS

Lists Docker, Docker Compose v2 (with explicit note distinguishing `docker compose` from legacy `docker-compose`), `curl`, and `jq`. Matches the design's FR-35 and `e2e-test.sh`'s prerequisite check.

### CR-3: Getting Started — PASS

- `cp .env.example .env` — present.
- `docker compose up --build` — present.
- `docker compose ps`, `docker compose logs`, `docker compose logs lambda`, `docker compose logs postgres` — present.
- `docker compose down` with explanation that the volume is preserved — present.
- `docker compose down -v` with explanation that the volume is removed — present.
- Distinct effects clearly documented.

### CR-4: Database Initialization Behavior — PASS

- Explains `/docker-entrypoint-initdb.d` mechanism.
- States `migrations/001_initial_schema.sql` is mounted as `01-schema.sql` — matches `docker-compose.yml` volume mount.
- States `local-env/init/02-seed.sql` is mounted as `02-seed.sql` — matches `docker-compose.yml` volume mount.
- Documents "only when the data directory is empty" behavior.
- Documents `docker compose down -v` as the reset path.

### CR-5: Invoking the Lambda Locally — PASS

**Deposit curl example:**
- URL: `http://localhost:9000/2015-03-31/functions/function/invocations` — correct.
- Envelope: API Gateway HTTP API v2 with `version`, `routeKey`, `rawPath`, `requestContext.http.*`, `headers`, `body`, `isBase64Encoded` — matches design's canonical shape.
- `body` is a JSON-encoded string (escaped), not a nested object — correct and explicitly called out.
- Uses `client_id` `00000000-0000-0000-0000-000000000001` — matches seed and e2e script.
- Uses `asset_id` `00000000-0000-0000-0000-000000000101` (CDB) — matches seed.
- Values: `amount` `"10"`, `unit_price` `"100.00"` — matches E2E scenario values.
- Placeholder `order_id` `<REPLACE-WITH-A-FRESH-UUID>` with `uuidgen` suggestion — correct.
- Pipes through `jq .` — present.
- Expected `statusCode` `201` — correct.

**Withdrawal curl example:**
- Same URL and envelope structure.
- Uses `instrument_id` `CDB-0001` — matches seed and e2e script.
- Uses `desired_value` `"250.00"` — matches E2E scenario values.
- Placeholder `order_id` — correct.
- Pipes through `jq .` — present.
- Expected `statusCode` `200` — correct.
- Notes that a deposit must exist first — helpful and accurate.

### CR-6: Seed Data Reference — PASS

**Clients table (10 rows):**
Cross-referenced against design and `02-seed.sql`. All 10 UUIDs (`00000000-0000-0000-0000-00000000000X`, X=1..10) and `external_id` values (`CLIENT-001`..`CLIENT-010`) match exactly. Same row order.

**Assets table (7 rows):**
Cross-referenced against design and `02-seed.sql`. All 7 rows match exactly on all four columns (`asset_id`, `product_type`, `instrument_id`, `asset_name`). UUIDs, instrument IDs, product types, and asset names are identical. Same row order.

### CR-7: Running the End-to-End Test — PASS

- Documents `./e2e-test.sh`.
- States deposit assertion `statusCode` `201` — matches `e2e-test.sh` line `invoke_and_assert "deposit" "$DEPOSIT_PAYLOAD" "201"`.
- States withdrawal assertion `statusCode` `200` — matches `e2e-test.sh` line `invoke_and_assert "withdrawal" "$WITHDRAW_PAYLOAD" "200"`.
- Documents PASS/FAIL per step — matches `invoke_and_assert` output.
- Documents trap-based teardown — matches `e2e-test.sh` EXIT trap.
- Documents exit code contract (0 success, non-zero failure) — correct.

### CR-8: No forbidden paths modified — PASS

The only file created by this step is `README.md`. No changes to `cmd/`, `internal/`, `go.mod`, `go.sum`, `migrations/`, `Dockerfile`, `.dockerignore`, `docker-compose.yml`, `.env`, `.env.example`, `local-env/`, `e2e-test.sh`, or `.agent-specstar/`.

---

## Cross-Reference Verification

| Check | Source | README | Match |
|---|---|---|---|
| Invoke URL | `e2e-test.sh` `INVOKE_URL` / `docker-compose.yml` port `9000:8080` | `http://localhost:9000/2015-03-31/functions/function/invocations` | exact |
| Deposit body fields | `e2e-test.sh` `DEPOSIT_BODY` | `client_id`, `asset_id`, `order_id`, `amount`, `unit_price` | exact |
| Withdrawal body fields | `e2e-test.sh` `WITHDRAW_BODY` | `client_id`, `instrument_id`, `order_id`, `desired_value` | exact |
| Deposit values | design E2E scenario | `amount="10"`, `unit_price="100.00"` | exact |
| Withdrawal values | design E2E scenario | `desired_value="250.00"` | exact |
| Schema mount path | `docker-compose.yml` | `01-schema.sql` | exact |
| Seed mount path | `docker-compose.yml` | `02-seed.sql` | exact |
| `.env.example` keys | `.env.example` | `cp .env.example .env` | correct |
| Client UUIDs | `02-seed.sql` | 10 rows | exact |
| Asset UUIDs/IDs/types/names | `02-seed.sql` | 7 rows | exact |

## Scope Check

README only. No scope creep. No forbidden paths touched.

## Clean Code Check

Not applicable — pure documentation, no executable code.
