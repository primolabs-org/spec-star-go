# Step 002 — Application layer: deposit service

## Objective

Implement `DepositService` in `internal/application/` with request/response DTOs, input validation, entity lookups, position creation, idempotency, and comprehensive unit tests.

## In Scope

- `DepositRequest` struct with fields: `ClientID`, `AssetID`, `OrderID`, `Amount`, `UnitPrice` (all strings, parsed internally).
- `DepositResponse` struct matching the response DTO contract in `design.md`: `PositionID`, `ClientID`, `AssetID`, `Amount`, `UnitPrice`, `TotalValue`, `CollateralValue`, `JudiciaryCollateralValue`, `PurchasedAt`, `CreatedAt`, `UpdatedAt` (all serialized as strings/RFC3339).
- `DepositService` struct with constructor accepting: `ClientRepository`, `AssetRepository`, `PositionRepository`, `ProcessedCommandRepository`, `UnitOfWork`.
- `Execute(ctx context.Context, req DepositRequest) (*DepositResponse, int, error)` method.
- Input validation (rules 1–8 from design.md): required fields, UUID parsing, decimal parsing, strictly positive amounts.
- Idempotency check via `ProcessedCommandRepository.FindByTypeAndOrderID("DEPOSIT", orderID)` before entity lookups.
- Idempotent replay: deserialize stored `responseSnapshot` into `DepositResponse`, return with `200`.
- Client lookup via `ClientRepository.FindByID`. `ErrNotFound` → `422` with `"client not found"`.
- Asset lookup via `AssetRepository.FindByID`. `ErrNotFound` → `422` with `"asset not found"`.
- Product type validation via `ValidateProductType(asset.ProductType())`. Failure → `422` with `"unsupported product type"`.
- Position creation via `domain.NewPosition(clientID, assetID, amount, unitPrice, time.Now().UTC())`.
- Atomic persistence within `UnitOfWork.Do`: `PositionRepository.Create` then serialize response to JSON then `ProcessedCommandRepository.Create`.
- Race handling: if `ProcessedCommandRepository.Create` returns `ErrDuplicate`, re-read via `FindByTypeAndOrderID` and return snapshot with `200`. If re-read fails, return `409`.
- Command type constant: `"DEPOSIT"`.
- Validation short-circuits on first failure.
- All validation errors return status `422`. New deposit returns `201`. Idempotent replay returns `200`. Unexpected errors return `500`.
- Comprehensive unit tests with mocked port interfaces.

## Out of Scope

- HTTP/API Gateway event parsing (step 003).
- Lambda bootstrap wiring (step 004).
- Structured logging.
- Observability instrumentation.

## Required Reads

- `internal/domain/position.go` — `NewPosition` signature and field accessors.
- `internal/domain/processed_command.go` — `NewProcessedCommand` signature - `ResponseSnapshot()` accessor.
- `internal/domain/errors.go` — `ValidationError`, `ErrNotFound`, `ErrDuplicate` definitions.
- `internal/domain/product_type.go` — `ValidateProductType` function.
- `internal/domain/asset.go` — `Asset.ProductType()` accessor.
- `internal/ports/unit_of_work.go` — `UnitOfWork` interface.
- `internal/ports/position_repository.go` — `PositionRepository` interface.
- `internal/ports/processed_command_repository.go` — `ProcessedCommandRepository` interface.
- `internal/ports/client_repository.go` — `ClientRepository` interface.
- `internal/ports/asset_repository.go` — `AssetRepository` interface.

## Allowed Write Paths

- `internal/application/deposit_service.go`
- `internal/application/deposit_service_test.go`

## Forbidden Paths

- `internal/domain/` — domain model is stable and not modified by this feature.
- `internal/ports/` — port interfaces are stable.
- `internal/adapters/` — adapter changes belong to other steps.
- `cmd/` — bootstrap belongs to step 004.

## Known Abstraction Opportunities

- A `toDepositResponse(position *domain.Position) *DepositResponse` helper within the service file to avoid duplicating Position-to-DTO mapping between the new-deposit path and the idempotent-replay-from-snapshot path.

## Allowed Abstraction Scope

- Private helpers within `deposit_service.go` only. No new packages, no new interfaces.

## Required Tests

All tests use mocked port interfaces (defined in the test file).

### Happy path
1. Valid deposit creates Position, persists ProcessedCommand, returns `201` with correct response fields.
2. Verify `DepositResponse` field values match the created Position (amount, unit_price, total_value, timestamps, UUIDs).

### Idempotency — replay
3. When `FindByTypeAndOrderID` returns an existing ProcessedCommand, return its deserialized snapshot with `200`. No `Create` calls made.

### Idempotency — concurrent race
4. When `ProcessedCommandRepository.Create` returns `ErrDuplicate`, the service re-reads via `FindByTypeAndOrderID` and returns the snapshot with `200`.
5. When `ProcessedCommandRepository.Create` returns `ErrDuplicate` AND the re-read fails, return `409`.

### Validation failures (all return `422`)
6. Missing `client_id` → `"client_id is required"`.
7. Invalid `client_id` (not a UUID) → `"invalid client_id"`.
8. Missing `asset_id` → `"asset_id is required"`.
9. Invalid `asset_id` (not a UUID) → `"invalid asset_id"`.
10. Missing `order_id` → `"order_id is required"`.
11. Missing `amount` → `"amount is required"`.
12. Invalid `amount` (not a decimal) → `"invalid amount"`.
13. Missing `unit_price` → `"unit_price is required"`.
14. Invalid `unit_price` (not a decimal) → `"invalid unit_price"`.
15. `amount` is zero → `"amount must be positive"`.
16. `amount` is negative → `"amount must be positive"`.
17. `unit_price` is zero → `"unit_price must be positive"`.
18. `unit_price` is negative → `"unit_price must be positive"`.

### Entity lookup failures (all return `422`)
19. `ClientRepository.FindByID` returns `ErrNotFound` → `"client not found"`.
20. `AssetRepository.FindByID` returns `ErrNotFound` → `"asset not found"`.
21. `ValidateProductType` fails for the asset's product type → `"unsupported product type"`.

### Unexpected errors (return `500`)
22. `ClientRepository.FindByID` returns an unexpected error → `500`.
23. `AssetRepository.FindByID` returns an unexpected error → `500`.
24. `ProcessedCommandRepository.FindByTypeAndOrderID` returns an unexpected error on initial check → `500`.
25. `UnitOfWork.Do` propagates an unexpected error from `PositionRepository.Create` → `500`.

## Coverage Requirement

100% line coverage on `internal/application/deposit_service.go`.

## Failure Model

- Validation errors → `422` with descriptive message. Explicit, not swallowed.
- `ErrNotFound` from Client/Asset → `422`. Treated as input validation failure.
- `ErrDuplicate` from ProcessedCommand.Create → handled by re-read. If re-read fails → `409`.
- Any other error → `500`. Propagated with context via `fmt.Errorf("...: %w", err)`.
- Errors wrapped with `%w` to preserve cause chain for `errors.Is` / `errors.As` inspection.

## Allowed Fallbacks

- `ErrDuplicate` race resolution via re-read is the only recovery path. This is an explicit design requirement, not a hidden fallback.

## Acceptance Criteria

1. `DepositService.Execute` implements the full orchestration flow from `design.md`.
2. All validation rules (1–11) are enforced in order, short-circuiting on first failure.
3. Idempotency check happens before entity lookups.
4. Client and Asset lookups happen outside `UnitOfWork.Do`.
5. Position and ProcessedCommand creation happen inside `UnitOfWork.Do`.
6. `DepositResponse` JSON tags use the field names from the response contract in `design.md`.
7. Decimal fields in `DepositResponse` serialize as strings.
8. `purchased_at` is set to `time.Now().UTC()` by the service, not the caller.
9. All test scenarios pass.
10. `go build ./...` succeeds.
11. `go vet ./...` produces no warnings on new files.

## Deferred Work

None.

## Escalation Conditions

- If `NewPosition` validation (`>= 0`) conflicts with the service's strictly-positive validation (`> 0`), it means the service check runs first and prevents the domain check from ever triggering for deposits. This is by design — do not modify `NewPosition`.
- If `ProcessedCommand.ResponseSnapshot()` returns a type that cannot be cleanly deserialized into `DepositResponse`, escalate.
