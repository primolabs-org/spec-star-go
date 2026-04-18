# Step 002 — Domain error types and value objects

## Goal

Define domain error types and the `ProductType` value object that all entities and ports depend on.

## Why

Entities (step 003) need error types for invariant violations and product type validation at construction time. Ports (step 004) need error types for repository failure classification. Establishing these foundational types first keeps subsequent steps focused.

## Depends on

Step 001.

## Required Reads

- `design.md` — "Domain Invariants", "Product Type Handling", "Optimistic Concurrency", "Idempotency", and "Resolved Decisions" sections.
- go-lambda-error-handling skill — domain error design rules.
- specstar-clean-code skill — naming, cognitive complexity, function design.

## In Scope

### Domain error types (`internal/domain/errors.go`)

Sentinel and typed errors that allow callers to classify failures using `errors.Is` and `errors.As`:

- **Validation error**: typed error carrying a message, used when entity invariants are violated at construction or mutation time.
- **Not found**: sentinel error for repository lookups that return no result.
- **Concurrency conflict**: sentinel error for optimistic concurrency failures on Position update.
- **Duplicate**: sentinel error for uniqueness constraint violations (ProcessedCommand idempotency key).

Error types must not depend on any infrastructure, AWS, HTTP, or pgx types.

### Product type value object (`internal/domain/product_type.go`)

- `ProductType` as a Go string type with a closed valid set: `CDB`, `LF`, `LCI`, `LCA`, `CRI`, `CRA`, `LFT`.
- A validation function that rejects any value outside the closed set, returning a validation error.
- Product types are metadata — no behavioral branching based on product type.

## Out of Scope

- Entity definitions (step 003).
- Repository port interfaces (step 004).
- Infrastructure error translation (step 007).
- HTTP status mapping or SQS error classification.

## Files to Create

- `internal/domain/errors.go`
- `internal/domain/errors_test.go`
- `internal/domain/product_type.go`
- `internal/domain/product_type_test.go`

## Forbidden Paths

- `internal/ports/`
- `internal/adapters/`
- `internal/platform/`
- `internal/application/`

## Required Tests

### `internal/domain/errors_test.go`

- Validation error is classifiable via `errors.As`.
- Validation error message is preserved through wrapping.
- Sentinel errors (`ErrNotFound`, `ErrConcurrencyConflict`, `ErrDuplicate`) are classifiable via `errors.Is` after wrapping with `fmt.Errorf("context: %w", err)`.

### `internal/domain/product_type_test.go`

- Each valid product type (`CDB`, `LF`, `LCI`, `LCA`, `CRI`, `CRA`, `LFT`) is accepted.
- Empty string is rejected with a validation error.
- Arbitrary invalid string (e.g., `"INVALID"`) is rejected with a validation error.
- Case sensitivity: lowercase or mixed-case variants of valid types are rejected (the set is exact uppercase).

## Coverage Requirement

100% on all new lines.

## Acceptance Criteria

- `go build ./...` succeeds.
- `go test ./internal/domain/...` passes with all tests green.
- Error types support `errors.Is` / `errors.As` classification after wrapping.
- Product type validation rejects invalid values with a validation error.
- No infrastructure imports in `internal/domain/`.

## Escalation Conditions

- If the error taxonomy needs additional categories not anticipated by the design, escalate.
