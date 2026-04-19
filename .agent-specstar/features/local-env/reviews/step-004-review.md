# Step 004 Review — e2e-test.sh

## Outcome: REJECTED

One required change. All other acceptance criteria pass.

---

## Per-Criterion Assessment

### CR-1: File exists at repo root, executable — PASS
`e2e-test.sh` exists at the repo root with `-rwxr-xr-x` permissions.

### CR-2: `set -euo pipefail` first, trap before compose up — PASS
Line 2: `set -euo pipefail`. Line 7: EXIT trap. Line 38: `docker compose up --build -d`. Order is correct.

### CR-3: Prerequisite check — PASS
Lines 11–18 verify `docker`, `docker compose` (via `docker compose version`), `curl`, `jq`. Missing tools produce a clear message and exit 1 before any container action.

### CR-4: Env loading with defaults — PASS
Lines 22–30 source `.env` if present, then export all five variables with defaults matching `.env.example` (`wallet`/`wallet`/`wallet`/`5432`/`9000`).

### CR-5: `docker compose up --build -d` — PASS
Line 38.

### CR-6: PostgreSQL readiness wait — PASS
`wait_for_postgres()` polls `docker compose exec -T postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"`, 1 s interval, 30 attempts, exits 1 on timeout.

### CR-7: Lambda RIE readiness wait — PASS
`wait_for_lambda()` POSTs a probe payload (`GET /probe` — an unknown path) to the invoke URL, 1 s interval, 30 attempts. `curl -fsS` succeeds on HTTP 200 from the RIE regardless of the embedded Lambda `statusCode`, which matches the design's readiness contract.

### CR-8: UUID fallback chain — PASS
`generate_uuid()` follows `uuidgen` → `/proc/sys/kernel/random/uuid` → `openssl rand -hex 16` (reformatted) → `exit 1`. uuidgen output is lowercased. Chain and failure behavior are correct.

### CR-9: Deposit invocation — PASS (payload/assertion correct)
Payload uses `client_id=00000000-…-000000000001`, `asset_id=00000000-…-000000000101`, `amount="10"`, `unit_price="100.00"`, fresh `order_id`. Body is double-encoded via `jq -Rs .`. Asserts `statusCode == 201`.

### CR-10: Withdrawal invocation — PASS (payload/assertion correct)
Payload uses same `client_id`, `instrument_id=CDB-0001`, `desired_value="250.00"`, fresh `order_id`. Body double-encoded. Asserts `statusCode == 200`.

### CR-11: Exit code contract — **FAIL** (fail-fast violation)
The script does exit 0 on success and non-zero on failure, and the trap fires in both cases. However, `invoke_and_assert` uses a `FAILURES` accumulator: on statusCode mismatch it increments a counter and **continues** to the next invocation. The step's Failure Model requires:

> *"Treat any timeout, transport failure, missing JSON field, or `statusCode` mismatch as a hard failure with a non-zero exit."*

The accumulator pattern:
1. Violates the step's explicit fail-fast contract — a mismatch must cause a hard failure, not deferred accounting.
2. Causes misleading cascading output — the withdrawal depends on a successful deposit; a deposit failure guarantees a spurious withdrawal failure.
3. Violates the repository-wide fail-fast instructions (fail-fast.instructions.md).
4. Introduces dead code: the `FAILURES` variable, its increment, and the final `if [[ $FAILURES -gt 0 ]]` block are unnecessary under fail-fast semantics.

### CR-12: No forbidden paths modified — PASS
Only `e2e-test.sh` was created. No changes to `cmd/`, `internal/`, `go.mod`, `go.sum`, `migrations/`, `Dockerfile`, `docker-compose.yml`, `.env.example`, `local-env/`, or `.agent-specstar/`.

### Additional: `|| true` only on trap — PASS
`|| true` appears only on line 7 inside the EXIT trap. No other error suppression.

### Additional: No "leave environment up" flag — PASS
No such flag exists.

### Additional: body double-encoding — PASS
Both payloads use `jq -Rs .` to produce the JSON-encoded string required by the `events.APIGatewayV2HTTPRequest` `body` field.

---

## Required Change

**In `invoke_and_assert`**: replace the `FAILURES` accumulator with an immediate `exit 1` on assertion failure. Remove the `FAILURES` variable, its increment, and the final check/summary block that depend on it.

The `PASS` echo and the final success message (`echo "All tests passed."`) can remain.
