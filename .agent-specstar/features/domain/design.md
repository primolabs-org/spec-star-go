# Domain Foundation

## Feature Summary

Establish the core domain model, persistence schema, repository contracts, and database access wiring for a fixed-income wallet microservice. This feature delivers the foundational layer — entities, invariants, migrations, and port interfaces — required before any command endpoints (deposit, withdraw) are implemented. The wallet is product-agnostic, treating all supported Brazilian fixed-income instruments with identical custody semantics.

## Glossary

- **Client**: Lightweight entity holding references to external domains. The wallet domain does not manage personal or sensitive client data.
- **Position**: A single deposit lot representing the client's holding for one asset. Contains amount, unit price, derived total value, and blocked-value components.
- **Asset**: A tradable fixed-income instrument identified by both a wallet-facing `asset_id` and a canonical `instrument_id` from the upstream catalog.
- **Instrument**: Canonical financial instrument identity used to group lots for withdrawal and position ownership, independent of a specific deposit order.
- **Product type**: High-level instrument family: `CDB`, `LF`, `LCI`, `LCA`, `CRI`, `CRA`, or `LFT`.
- **B3 identifier**: The client's unique custody or exchange identifier in Brazilian market infrastructure.
- **Unit price**: Monetary value of a single unit at purchase time.
- **Total value**: `amount × unit_price` — the gross value of a holding.
- **Available value**: `total_value - collateral_value - judiciary_collateral_value`.

## Domain Model

### Entities

#### Client

| Field         | Type      | Notes                                    |
|---------------|-----------|------------------------------------------|
| `client_id`   | UUID      | PK                                       |
| `external_id` | string    | B3 identifier or upstream custody ID     |
| `created_at`  | timestamp | Set at creation, immutable               |

#### Asset

| Field                | Type        | Notes                                           |
|----------------------|-------------|-------------------------------------------------|
| `asset_id`           | UUID        | PK                                              |
| `instrument_id`      | string      | Canonical ID for lot grouping                   |
| `product_type`       | enum string | Constrained to supported families               |
| `offer_id`           | string?     | Optional offer/distribution identifier           |
| `emission_entity_id` | string      | Emitter entity identifier                        |
| `issuer_document_id` | string?     | Optional upstream issuer reference               |
| `market_code`        | string?     | Optional B3 ticker/reference                     |
| `asset_name`         | string      | Human-readable name                              |
| `issuance_date`      | date        | Date of issuance                                 |
| `maturity_date`      | date        | Maturity date                                    |
| `created_at`         | timestamp   | Set at creation, immutable                       |

#### Position (deposit lot)

| Field                       | Type         | Notes                                        |
|-----------------------------|--------------|----------------------------------------------|
| `position_id`               | UUID         | PK                                           |
| `client_id`                 | UUID         | FK → Client                                  |
| `asset_id`                  | UUID         | FK → Asset                                   |
| `amount`                    | decimal(18,6)| Units held                                   |
| `unit_price`                | decimal(20,8)| Price per unit at purchase time               |
| `total_value`               | decimal(20,8)| Derived: `amount × unit_price`               |
| `collateral_value`          | decimal(20,8)| Blocked as collateral                         |
| `judiciary_collateral_value`| decimal(20,8)| Blocked as judiciary collateral               |
| `created_at`                | timestamp    | Set at creation, immutable                    |
| `updated_at`                | timestamp    | Updated on every mutation                     |
| `purchased_at`              | timestamp    | Time of original purchase                     |
| `row_version`               | integer      | Optimistic concurrency token, starts at 1     |

#### ProcessedCommand

| Field               | Type      | Notes                                                  |
|---------------------|-----------|--------------------------------------------------------|
| `command_id`        | UUID      | PK                                                     |
| `command_type`      | string    | e.g. `DEPOSIT`, `WITHDRAW`                              |
| `order_id`          | string    | External idempotency key                                |
| `client_id`         | UUID      | Client that owns the command                            |
| `response_snapshot` | JSONB     | Persisted result for deterministic replay               |
| `created_at`        | timestamp | Set at creation, immutable                              |

### Relationships

- Client 1 → N Position
- Asset 1 → N Position
- Client 1 → N ProcessedCommand

### Domain Invariants (per Position)

- `amount >= 0`
- `unit_price >= 0`
- `total_value == amount × unit_price` (derived, never set independently)
- `collateral_value >= 0`
- `judiciary_collateral_value >= 0`
- `available_value == total_value - collateral_value - judiciary_collateral_value` (computed, not stored)

These invariants are enforced at entity construction and mutation time in the domain layer. The database provides a secondary enforcement layer via CHECK constraints.

## Product Type Handling

### Application level

`product_type` is modeled as a Go string type with a closed set of valid values: `CDB`, `LF`, `LCI`, `LCA`, `CRI`, `CRA`, `LFT`. Validation happens at entity construction — attempting to create an Asset with an unsupported product type fails immediately.

No product-specific behavioral branching exists in the domain or application layers. Product type is metadata for validation, routing, and future extensibility only.

### Database level

The `product_type` column uses a PostgreSQL `CHECK` constraint against the supported values. This provides defense-in-depth but the domain layer is the primary enforcement point.

A PostgreSQL `ENUM` type is intentionally avoided because altering enums in PostgreSQL requires DDL that is awkward in migrations and under concurrent connections. A `VARCHAR` with a `CHECK` constraint is simpler to extend.

## Repository Contracts

Port interfaces needed for future deposit and withdraw command flows. These live in `internal/ports/`.

### ClientRepository

- Find client by ID
- Create client

### AssetRepository

- Find asset by ID
- Find asset by instrument ID
- Create asset

### PositionRepository

- Find position by ID (with row_version for concurrency)
- Find positions by client ID and asset ID
- Find positions by client ID and instrument ID, ordered by purchased_at (requires join to Asset)
- Create position
- Update position (with optimistic concurrency check on row_version)

### ProcessedCommandRepository

- Find processed command by command type and order ID (idempotency lookup)
- Create processed command

### Unit of Work / Transaction Support

Repository operations for future withdraw flows must support atomic multi-row mutations within a single database transaction. The port layer defines a transaction boundary abstraction (e.g., a function that receives a `context.Context`-scoped transaction) so the application layer can orchestrate multi-step persistence without depending on `pgx` types directly.

## Persistence Schema

### Table: `clients`

```sql
CREATE TABLE clients (
    client_id       UUID        PRIMARY KEY,
    external_id     VARCHAR     NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Table: `assets`

```sql
CREATE TABLE assets (
    asset_id           UUID        PRIMARY KEY,
    instrument_id      VARCHAR     NOT NULL,
    product_type       VARCHAR     NOT NULL CHECK (product_type IN ('CDB','LF','LCI','LCA','CRI','CRA','LFT')),
    offer_id           VARCHAR,
    emission_entity_id VARCHAR     NOT NULL,
    issuer_document_id VARCHAR,
    market_code        VARCHAR,
    asset_name         VARCHAR     NOT NULL,
    issuance_date      DATE        NOT NULL,
    maturity_date      DATE        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_assets_instrument_id ON assets (instrument_id);
```

### Table: `positions`

```sql
CREATE TABLE positions (
    position_id               UUID           PRIMARY KEY,
    client_id                 UUID           NOT NULL REFERENCES clients (client_id),
    asset_id                  UUID           NOT NULL REFERENCES assets (asset_id),
    amount                    NUMERIC(18,6)  NOT NULL CHECK (amount >= 0),
    unit_price                NUMERIC(20,8)  NOT NULL CHECK (unit_price >= 0),
    total_value               NUMERIC(20,8)  NOT NULL CHECK (total_value >= 0),
    collateral_value          NUMERIC(20,8)  NOT NULL DEFAULT 0 CHECK (collateral_value >= 0),
    judiciary_collateral_value NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (judiciary_collateral_value >= 0),
    created_at                TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ    NOT NULL DEFAULT now(),
    purchased_at              TIMESTAMPTZ    NOT NULL,
    row_version               INTEGER        NOT NULL DEFAULT 1 CHECK (row_version > 0),
    CHECK (total_value = amount * unit_price)
);

CREATE INDEX idx_positions_client_asset ON positions (client_id, asset_id);
```

Note: Position lookup by `(client_id, instrument_id, purchased_at)` requires a join to `assets`. The `idx_positions_client_asset` index combined with `idx_assets_instrument_id` supports this access pattern. A dedicated composite index is not viable without denormalization since `instrument_id` lives on `assets`.

### Table: `processed_commands`

```sql
CREATE TABLE processed_commands (
    command_id        UUID        PRIMARY KEY,
    command_type      VARCHAR     NOT NULL,
    order_id          VARCHAR     NOT NULL,
    client_id         UUID        NOT NULL,
    response_snapshot JSONB       NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_processed_commands_type_order ON processed_commands (command_type, order_id);
```

## Decimal Handling

### Problem

The domain requires exact decimal arithmetic with scales up to `NUMERIC(20,8)`. Go's `float64` is insufficient — IEEE 754 introduces rounding errors unacceptable for financial calculations. The `total_value = amount × unit_price` invariant must hold exactly.

### Approach: `shopspring/decimal`

`github.com/shopspring/decimal` is approved for this feature. It is the de facto standard for arbitrary-precision decimal arithmetic in Go financial systems. It maps cleanly to PostgreSQL `NUMERIC` via string-based scanning, preserves exact scale, and provides correct arithmetic operations.

## Optimistic Concurrency

`row_version` on Position is an integer column starting at 1, incremented on every update.

**Write path**:
1. Read the position, capturing `row_version`.
2. Apply domain mutations.
3. Issue `UPDATE ... SET row_version = row_version + 1 ... WHERE position_id = $1 AND row_version = $2`.
4. If zero rows affected, the position was modified concurrently — return a conflict error.

The repository contract surfaces this as a typed concurrency-conflict error so the application layer can distinguish it from other failures.

## Idempotency

`ProcessedCommand` supports idempotent command processing for future deposit and withdraw flows.

**Flow**:
1. Before executing a command, query `processed_commands` by `(command_type, order_id)`.
2. If a record exists, return the stored `response_snapshot` without re-executing.
3. If no record exists, execute the command and insert the `ProcessedCommand` record atomically within the same transaction as the domain mutation.

The `UNIQUE` index on `(command_type, order_id)` provides database-level protection against race conditions — a concurrent insert for the same key will fail with a unique constraint violation, which the adapter surfaces as a conflict.

`response_snapshot` is stored as JSONB, containing enough data to reconstruct the original command response deterministically.

## Migration Strategy

- Plain numbered SQL files in `migrations/` directory: `001_initial_schema.sql`, `002_...`, etc.
- No migration framework or Go migration tooling. Migrations are applied by an external CI/CD resource outside the Lambda application.
- The initial migration creates all four tables, indexes, and constraints in a single file.
- Each migration file contains idempotent-safe DDL where practical (e.g., `CREATE TABLE IF NOT EXISTS` is acceptable for initial creation, though not required if migrations are applied in order).
- Migration ordering: `clients` → `assets` → `positions` → `processed_commands` (respecting FK dependencies).

## Connection Management

`pgxpool` is used for database connection pooling, configured for Lambda's execution model:

- **Max connections**: 2–5. Lambda instances are single-concurrent; a small pool handles in-flight queries plus one spare. The exact value is sourced from environment configuration.
- **Min connections**: 0. Lambda instances freeze between invocations; idle connections may be closed by Aurora. A zero minimum avoids wasting resources on frozen instances.
- **Max connection lifetime**: Bounded (e.g., 5 minutes) to rotate connections and avoid stale TCP sessions after Lambda freeze/thaw.
- **Max connection idle time**: Short (e.g., 30 seconds) for the same freeze/thaw reason.
- **Health check period**: Enabled to detect connections broken during freeze.
- **Pool initialization**: Created once in the Lambda cold-start path (outside the handler), reused across warm invocations.

Connection string and pool parameters are sourced from environment variables.

## Functional Requirements

1. Define four domain entities: Client, Asset, Position, ProcessedCommand with all fields listed in the domain model.
2. `total_value` is derived as `amount × unit_price` — computed at entity construction and mutation, never set independently.
3. `available_value` is computed as `total_value - collateral_value - judiciary_collateral_value` — a domain method, not a stored field.
4. `row_version` on Position supports optimistic concurrency as described above.
5. Domain entity constructors and mutators enforce all invariants at the domain level; invalid state is rejected immediately.
6. `product_type` on Asset is validated against the closed set of supported values at both domain and database levels.
7. Repository port interfaces are defined for all four entities with operations required by future deposit/withdraw flows.
8. A transaction boundary abstraction is defined in the port layer to support atomic multi-row persistence.
9. SQL migrations create the initial schema with all tables, columns, types, constraints, and indexes.
10. Indexes support the required access patterns:
    - Position lookup by `(client_id, asset_id)`
    - Asset lookup by `instrument_id`
    - Position selection by `(client_id, instrument_id, purchased_at)` via join
    - Idempotency lookup by `(command_type, order_id)`
11. Product-agnostic behavior: no product-type-specific branching in domain or repository logic. All supported instruments share identical custody semantics.
12. Withdrawal grouping (future) uses `instrument_id`, not product-specific logic.

## Non-Functional Requirements

1. Decimal precision: `amount` as NUMERIC(18,6), `unit_price` and all value fields as NUMERIC(20,8). Go representation must preserve exact precision with no floating-point rounding.
2. Domain invariants enforced at the entity/domain level, not only at the database level.
3. Target write workload: sustained 1,000 req/min, short burst 100 req/s.
4. Target API latency budget (future endpoints): p95 < 200 ms, p99 < 400 ms.
5. Schema and repository design support atomic multi-row withdrawals in a single transaction.
6. Repository operations compatible with Aurora PostgreSQL connection pooling and Lambda execution model.
7. Minimal cold-start impact: pool initialization is lightweight, connections are established lazily.

## Out of Scope

- API endpoints (HTTP handlers, request/response DTOs, routing)
- Deposit or withdraw command logic
- Read models or projections
- Yield, accrual, mark-to-market, or tax calculations
- Client master-data management
- Asset master-data authoring
- Settlement, clearing, or ledger entries
- Seed data

## Constraints and Allowed Libraries

### Constraints

- Go 1.24+
- AWS Lambda compute model
- HTTP trigger via API Gateway HTTP API (initial trigger)
- AWS Aurora PostgreSQL backing store
- Hexagonal architecture: domain and ports have zero infrastructure dependencies
- No ORM — SQL-first persistence
- No heavy HTTP frameworks, DI frameworks, mediator frameworks, or reflection-heavy validation libraries
- Fail-fast behavior: reject invalid input immediately, do not guess defaults

### Allowed libraries

| Library                            | Status                                |
|------------------------------------|---------------------------------------|
| Go standard library                | Allowed                               |
| `github.com/aws/aws-lambda-go`    | Allowed                               |
| `github.com/jackc/pgx/v5`         | Allowed                               |
| `github.com/jackc/pgx/v5/pgxpool` | Allowed                               |
| `github.com/shopspring/decimal`    | Allowed                               |
| `github.com/google/uuid`           | Allowed                               |
| `go.opentelemetry.io/otel`        | Optional, only if enabled by task     |
| Testing helper libraries           | Optional, only if enabled by task     |

## Resolved Decisions

1. **Decimal library**: `shopspring/decimal` approved.
2. **UUID generation**: `google/uuid` approved. UUIDs are generated by entity constructors.
3. **Migration runner**: External CI/CD. No Go migration tooling in scope.
4. **`response_snapshot` typing**: Opaque `[]byte` (JSON). Command implementations define structure later.

## Success Criteria

- All four domain entities are defined with complete field sets and enforced invariants.
- `total_value` is always computed from `amount × unit_price` — never set independently.
- Optimistic concurrency on Position is functional via `row_version`.
- Idempotency mechanism via ProcessedCommand is structurally complete.
- Repository port interfaces cover all operations required by future deposit and withdraw flows.
- SQL migrations create the full schema with correct types, constraints, and indexes.
- Product type validation is enforced at both domain and database levels.
- No product-specific behavioral branching exists in domain code.
- All decimal fields maintain exact precision without floating-point rounding.
- Connection pooling is configured appropriately for Lambda execution model.
- Domain and port layers have zero dependencies on AWS or infrastructure packages.
