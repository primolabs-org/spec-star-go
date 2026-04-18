# Step 004 — Repository port interfaces

## Goal

Define repository contracts for all four entities and the transaction boundary abstraction as port interfaces.

## Why

Port interfaces decouple domain and application logic from persistence infrastructure. The adapter implementations (step 007) fulfill these contracts. The transaction boundary abstraction enables future atomic multi-row operations (withdraw flows) without leaking `pgx` types into application code.

## Depends on

Step 003 (port interfaces reference domain entity types).

## Required Reads

- `design.md` — "Repository Contracts" section for method signatures, "Optimistic Concurrency" and "Idempotency" sections for behavioral contracts, "Unit of Work / Transaction Support" description.
- go-lambda-error-handling skill — repository contracts surface typed domain errors.
- go-aws-lambda-microservice-hexagonal skill — ports have zero infrastructure dependencies.

## In Scope

### ClientRepository (`internal/ports/client_repository.go`)

- Find client by ID → returns domain Client or `ErrNotFound`.
- Create client → persists a new Client.

### AssetRepository (`internal/ports/asset_repository.go`)

- Find asset by ID → returns domain Asset or `ErrNotFound`.
- Find asset by instrument ID → returns domain Asset or `ErrNotFound`.
- Create asset → persists a new Asset.

### PositionRepository (`internal/ports/position_repository.go`)

- Find position by ID → returns domain Position (with `row_version`) or `ErrNotFound`.
- Find positions by client ID and asset ID → returns a slice (may be empty).
- Find positions by client ID and instrument ID, ordered by `purchased_at` ascending → returns a slice (may be empty). Note: this requires a join to Asset at the adapter level, but the port contract expresses the business need.
- Create position → persists a new Position.
- Update position → applies mutation with optimistic concurrency check on `row_version`. Returns `ErrConcurrencyConflict` if the version does not match.

### ProcessedCommandRepository (`internal/ports/processed_command_repository.go`)

- Find processed command by command type and order ID → returns domain ProcessedCommand or `ErrNotFound`. This is the idempotency lookup.
- Create processed command → persists a new ProcessedCommand. Returns `ErrDuplicate` if the `(command_type, order_id)` unique constraint is violated.

### Transaction boundary (`internal/ports/unit_of_work.go`)

- A `UnitOfWork` interface that accepts a `context.Context` and a function, executes the function within a database transaction, commits on success, and rolls back on failure.
- The interface must not import or reference `pgx`, `pgxpool`, or any infrastructure types.
- The function parameter receives a `context.Context` that carries the transaction scope. Repository implementations detect whether they are operating within a transaction by inspecting the context.

### Error contract

- All "not found" cases return `domain.ErrNotFound` (wrapped with context).
- Position update concurrency failures return `domain.ErrConcurrencyConflict`.
- ProcessedCommand duplicate insert returns `domain.ErrDuplicate`.
- All methods accept `context.Context` as the first parameter.

## Out of Scope

- ANY implementation of these interfaces.
- SQL queries or pgx code.
- Database connection management.
- Application use cases or command handlers.

## Files to Create

- `internal/ports/client_repository.go`
- `internal/ports/asset_repository.go`
- `internal/ports/position_repository.go`
- `internal/ports/processed_command_repository.go`
- `internal/ports/unit_of_work.go`

## Forbidden Paths

- `internal/adapters/`
- `internal/platform/`
- `internal/application/`
- `internal/domain/` — do not modify domain files from step 002/003.

## Required Tests

None. These are interface definitions with no executable behavior. Compilation verification is sufficient.

A compile-time interface satisfaction check (e.g., `var _ ports.ClientRepository = (*postgresClientRepo)(nil)`) will be added in step 007 when implementations exist.

## Coverage Requirement

N/A — no executable code.

## Acceptance Criteria

- `go build ./...` succeeds.
- All interfaces reference only domain types and standard library types (`context.Context`, `error`).
- No `pgx`, `pgxpool`, `aws`, or infrastructure imports in `internal/ports/`.
- Method signatures match the contracts described in design.md "Repository Contracts" section.
- `UnitOfWork` interface is defined with no infrastructure type leakage.

## Escalation Conditions

- If any repository method requires a domain type or parameter not yet defined in step 003, escalate to determine whether to extend the entity or adjust the contract.
