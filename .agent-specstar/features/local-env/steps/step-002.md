# Step 002 - PostgreSQL Seed Data

## Metadata
- Feature: local-env
- Step: step-002
- Status: pending
- Depends On: none
- Last Updated: 2026-04-19

## Objective

Create the deterministic local seed file `local-env/init/02-seed.sql` that inserts the 10 clients and 7 assets defined in `.agent-specstar/features/local-env/design.md` § Seed data, designed to be mounted alongside the existing `migrations/001_initial_schema.sql` (which is reused as `01-schema.sql` by the next step).

## In Scope

- New seed file `local-env/init/02-seed.sql` with the exact UUIDs, `external_id` values, `instrument_id` values, `product_type` values, fixed `issuance_date`/`maturity_date`, and `emission_entity_id` listed in the design.
- `INSERT ... ON CONFLICT DO NOTHING` semantics on both `clients` and `assets` so the script is safely re-runnable manually against an existing database.

## Out of Scope

- Schema definition. `migrations/001_initial_schema.sql` is the source of truth and must not be copied or modified.
- Wiring the file into Compose (covered in step 003).
- Any seed data for `positions` or `processed_commands` (those are produced by the deposit/withdrawal flows themselves).

## Required Reads

- `.agent-specstar/features/local-env/design.md` — sections: Seed data, Constraints and Assumptions, Failure Model.
- `migrations/001_initial_schema.sql` — to confirm column names, NOT NULL columns, the `product_type` `CHECK` constraint, and the `idx_processed_commands_type_order` unique index (for awareness; not seeded).

## Allowed Write Paths

- `local-env/init/02-seed.sql`

## Forbidden Paths

- `cmd/**`
- `internal/**`
- `go.mod`, `go.sum`
- `migrations/**`
- `Dockerfile`, `.dockerignore`
- `docker-compose.yml`
- `.env`, `.env.example`
- `e2e-test.sh`
- `README.md`
- `.agent-specstar/**`

## Known Abstraction Opportunities

None. A flat sequence of `INSERT` statements is the clearest form for a seed file.

## Allowed Abstraction Scope

None. Do not introduce PL/pgSQL functions, generated `INSERT ... SELECT` from a `VALUES` table, or per-environment toggles.

## Required Tests

No automated tests. Validation is performed end-to-end in step 004 (`e2e-test.sh`), which exercises a seeded client and a seeded asset and therefore implicitly proves both the schema mount and this seed succeeded.

A reviewer-level local check that may be performed manually: starting `postgres:16-alpine` against the schema and this seed (e.g., via `psql -f`) must complete without errors and produce 10 rows in `clients` and 7 rows in `assets`.

## Coverage Requirement

This step changes **zero Go executable lines**. Go-side coverage rules do not apply. SQL correctness is gated by the e2e flow in step 004.

## Failure Model

Fail-fast. The script must surface any constraint violation immediately and abort PostgreSQL's init phase. Do not use `EXCEPTION WHEN OTHERS` blocks, do not wrap inserts in `DO $$ ... $$` constructs that swallow errors, and do not use savepoints to skip failures.

## Allowed Fallbacks

- none

## Acceptance Criteria

1. File exists at `local-env/init/02-seed.sql`.
2. Inserts exactly 10 rows into `clients` with `client_id` values `00000000-0000-0000-0000-000000000001` through `…010` and corresponding `external_id` values `CLIENT-001`..`CLIENT-010`, in that order.
3. Inserts exactly 7 rows into `assets` with `asset_id` values `00000000-0000-0000-0000-000000000101` through `…107`, one per `product_type` in the order `CDB, LF, LCI, LCA, CRI, CRA, LFT`, each with `instrument_id` `<TYPE>-0001`, `asset_name` `<TYPE> Local Test`, `emission_entity_id` `EMISSOR-LOCAL`, `issuance_date` `2026-01-01`, `maturity_date` `2030-01-01`. Optional columns are omitted (left `NULL`).
4. Both inserts use `ON CONFLICT (...) DO NOTHING` keyed on the primary key.
5. No statement modifies the schema. No `CREATE`, `ALTER`, `DROP`, or extension statements are present.
6. The file is self-contained and does not reference the schema script or assume any prior session state beyond a freshly initialized database from `migrations/001_initial_schema.sql`.
7. No file outside Allowed Write Paths is modified.

## Deferred Work

- none

## Escalation Conditions

- A future schema change adds a NOT NULL column without a default to `clients` or `assets`. The seed will need to be updated; escalate so the design and seed are revised together.
