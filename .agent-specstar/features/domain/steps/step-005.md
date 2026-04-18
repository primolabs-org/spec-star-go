# Step 005 — SQL migrations

## Goal

Create the initial database schema migration with all tables, constraints, indexes, and foreign keys for Aurora PostgreSQL.

## Why

The schema is required before outbound adapters (step 007) can be integration-tested and before the service can be deployed. The migration encodes the persistence shape that matches the domain model and supports the access patterns defined in the repository contracts.

## Depends on

Step 004 (the migration should match the repository contracts). In practice, step 005 has no Go code dependency — it can be implemented in parallel with step 004 if the design.md schema is used as the source of truth.

## Required Reads

- `design.md` — "Persistence Schema" section for complete DDL, "Migration Strategy" section for file naming and ordering.

## In Scope

- A single migration file creating all four tables in FK-dependency order: `clients` → `assets` → `positions` → `processed_commands`.
- All column types, `NOT NULL` constraints, `DEFAULT` values, `CHECK` constraints, primary keys, and foreign keys as specified in design.md.
- All indexes as specified in design.md:
  - `idx_assets_instrument_id` on `assets (instrument_id)`.
  - `idx_positions_client_asset` on `positions (client_id, asset_id)`.
  - `idx_processed_commands_type_order` as `UNIQUE` on `processed_commands (command_type, order_id)`.
- `product_type` uses a `VARCHAR` column with a `CHECK` constraint (not a PostgreSQL `ENUM` type), as specified in design.md.
- The `CHECK (total_value = amount * unit_price)` constraint on `positions`.

## Out of Scope

- Go migration tooling or runner code.
- Seed data.
- Rollback / down migration scripts.
- Additional indexes beyond what design.md specifies.
- Partitioning, RLS, or Aurora-specific extensions.

## Files to Create

- `migrations/001_initial_schema.sql`

## Forbidden Paths

- `internal/` — no Go code changes.
- `cmd/` — no Go code changes.

## Required Tests

No Go tests. The migration file is validated by:

1. Review against design.md "Persistence Schema" section for completeness.
2. Successful execution against a PostgreSQL instance (manual or CI verification).

## Coverage Requirement

N/A — SQL file, not Go code.

## Acceptance Criteria

- File `migrations/001_initial_schema.sql` exists.
- All four tables are created in FK-dependency order.
- All columns, types, constraints, defaults, and indexes match design.md "Persistence Schema" exactly.
- The file is valid PostgreSQL DDL that executes without errors on a clean Aurora PostgreSQL or standard PostgreSQL 14+ instance.
- `product_type` constraint uses `CHECK`, not `ENUM`.
- `total_value = amount * unit_price` CHECK constraint is present on `positions`.
- Unique index on `processed_commands (command_type, order_id)` is present.

## Escalation Conditions

- If the design.md schema has inconsistencies between the "Domain Model" field list and the "Persistence Schema" DDL, escalate.
