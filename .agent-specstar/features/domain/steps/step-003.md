# Step 003 — Domain entities

## Goal

Implement Client, Asset, Position, and ProcessedCommand entities with constructor validation, invariant enforcement, reconstitution support, and controlled mutation on Position.

## Why

The domain entities are the core model that all other layers depend on. Constructors enforce invariants at creation time (fail-fast). Reconstitution functions allow loading entities from persistence without re-generating derived values. Position mutation methods maintain invariants during state changes and manage the optimistic concurrency token.

## Depends on

Step 002 (domain error types and ProductType).

## Required Reads

- `design.md` — "Domain Model" section for complete field lists, "Domain Invariants" section for enforcement rules, "Decimal Handling" section for `shopspring/decimal` usage, "Optimistic Concurrency" section for row_version semantics.
- `clarifications.md` — `response_snapshot` is opaque `[]byte`, UUID generation uses `google/uuid`.
- go-lambda-error-handling skill — domain errors are transport-agnostic, no logging in domain code.
- specstar-clean-code skill — small focused functions, cognitive complexity <= 15.

## In Scope

### Client (`internal/domain/client.go`)

- Creation constructor: accepts `external_id`, generates `client_id` (UUID), sets `created_at`. Validates required fields.
- Reconstitution function: accepts all fields for loading from persistence.
- Exported getters for all fields (unexported struct fields).

### Asset (`internal/domain/asset.go`)

- Creation constructor: accepts all required fields, generates `asset_id` (UUID), sets `created_at`. Validates required fields and `product_type` against the closed set using the ProductType validator from step 002.
- Reconstitution function: accepts all fields for loading from persistence.
- Exported getters for all fields.

### Position (`internal/domain/position.go`)

- Creation constructor: accepts `client_id`, `asset_id`, `amount`, `unit_price`, `purchased_at`, generates `position_id` (UUID). Derives `total_value = amount × unit_price`. Sets `collateral_value` and `judiciary_collateral_value` to zero. Sets `row_version` to 1. Sets `created_at` and `updated_at`. Validates all invariants from design.md.
- Reconstitution function: accepts all fields for loading from persistence. Validates structural integrity (non-negative values, total_value consistency).
- `AvailableValue()` method: returns `total_value - collateral_value - judiciary_collateral_value` (computed, not stored).
- Mutation methods for fields that change during business operations (amount, collateral values). Each mutation re-derives `total_value` when amount changes, re-validates all invariants, increments `row_version`, and updates `updated_at`.
- `RowVersion()` getter for optimistic concurrency support in adapters.
- All decimal fields use `shopspring/decimal.Decimal`.

### ProcessedCommand (`internal/domain/processed_command.go`)

- Creation constructor: accepts `command_type`, `order_id`, `client_id`, `response_snapshot` (opaque `[]byte`), generates `command_id` (UUID), sets `created_at`. Validates required fields and non-empty `response_snapshot`.
- Reconstitution function: accepts all fields for loading from persistence.
- Exported getters for all fields.

### Design constraints (from design.md, not duplicated here)

- Refer to design.md "Domain Invariants" for the complete invariant list.
- Refer to design.md "Decimal Handling" for precision rules.
- Domain code must have ZERO infrastructure imports (`pgx`, `aws-lambda-go`, etc.).
- Domain code must not log directly.

## Out of Scope

- Repository port interfaces (step 004).
- Persistence, SQL, or adapter concerns.
- Deposit, withdraw, or any command/use-case logic.
- Application-level orchestration of mutations.
- Yield, accrual, or mark-to-market calculations.

## Files to Create

- `internal/domain/client.go`
- `internal/domain/client_test.go`
- `internal/domain/asset.go`
- `internal/domain/asset_test.go`
- `internal/domain/position.go`
- `internal/domain/position_test.go`
- `internal/domain/processed_command.go`
- `internal/domain/processed_command_test.go`

## Forbidden Paths

- `internal/ports/`
- `internal/adapters/`
- `internal/platform/`
- `internal/application/`

## Required Tests

### `internal/domain/client_test.go`

- Valid creation: all fields populated, `client_id` is a valid UUID, `created_at` is set.
- Missing `external_id`: returns validation error.

### `internal/domain/asset_test.go`

- Valid creation: all required fields populated, `asset_id` is a valid UUID.
- Invalid `product_type`: returns validation error.
- Missing required fields (`instrument_id`, `emission_entity_id`, `asset_name`, `issuance_date`, `maturity_date`): each returns validation error.
- Optional fields (`offer_id`, `issuer_document_id`, `market_code`) accepted as empty.

### `internal/domain/position_test.go`

- Valid creation: `total_value` derived correctly as `amount × unit_price`.
- Zero amount: accepted (valid — position can be depleted).
- Negative amount: returns validation error.
- Negative `unit_price`: returns validation error.
- `AvailableValue()`: returns `total_value - collateral_value - judiciary_collateral_value`.
- `AvailableValue()` with zero collaterals: equals `total_value`.
- Mutation: amount change re-derives `total_value`, increments `row_version`, updates `updated_at`.
- Mutation: negative collateral rejected with validation error.
- Mutation: collateral values exceeding `total_value` — clarify whether this is an invariant (design.md does not list this as a constraint, so accept for now).
- `row_version` starts at 1 on creation.
- Reconstitution: all fields populated correctly, `total_value` consistency validated.
- Decimal precision: verify that `amount × unit_price` produces exact results (no floating-point drift) using `shopspring/decimal`.

### `internal/domain/processed_command_test.go`

- Valid creation: all fields populated, `command_id` is a valid UUID.
- Missing `command_type`: returns validation error.
- Missing `order_id`: returns validation error.
- Empty `response_snapshot`: returns validation error.

## Coverage Requirement

100% on all new lines.

## Known Abstraction Opportunities

- If constructor validation becomes repetitive across entities, a small validation helper within the domain package may reduce duplication. Only extract if repetition is clear after implementing all four entities.

## Acceptance Criteria

- `go build ./...` succeeds.
- `go test ./internal/domain/...` passes with all tests green.
- `total_value` is always derived from `amount × unit_price` — never independently settable.
- `AvailableValue()` is computed, not stored.
- All invariants from design.md are enforced at construction and mutation time.
- `row_version` increments on every Position mutation.
- No infrastructure imports in `internal/domain/`.
- No direct logging in domain code.
- Cognitive complexity per function <= 15.

## Deferred Work

- Position mutation methods for specific business operations (e.g., deposit, withdraw) beyond generic field-level mutations are deferred to the command handler feature.

## Escalation Conditions

- If `total_value = amount × unit_price` cannot be enforced exactly with `shopspring/decimal` due to scale or precision issues, escalate.
- If the entity field list in design.md has ambiguities (e.g., which fields are truly required vs optional), escalate.
