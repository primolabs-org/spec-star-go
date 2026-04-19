# Step 001 — Application layer: WithdrawService

## Objective

Implement `WithdrawService` in `internal/application/` with request/response DTOs, typed application errors, input validation, FIFO lot selection with two-pass algorithm, idempotency, and comprehensive unit tests.

## In Scope

- `WithdrawRequest` struct with fields: `ClientID`, `ProductAssetID`, `OrderID`, `DesiredValue`, `IfMatch` (all strings, parsed internally).
- `WithdrawResponse` struct with field `AffectedPositions []AffectedPosition`.
- `AffectedPosition` struct matching the response DTO contract in `design.md`: `PositionID`, `ClientID`, `AssetID`, `Amount`, `UnitPrice`, `TotalValue`, `CollateralValue`, `JudiciaryCollateralValue`, `PurchasedAt`, `CreatedAt`, `UpdatedAt` (all serialized as strings/RFC3339).
- `InsufficientPositionError` typed error — implements `error`, message: `"insufficient position"`. Inspectable via `errors.As`.
- `ConcurrencyConflictError` typed error — implements `error`, message: `"concurrency conflict"`. Inspectable via `errors.As`.
- `WithdrawService` struct with constructor accepting: `ClientRepository`, `PositionRepository`, `ProcessedCommandRepository`, `UnitOfWork`. Note: no `AssetRepository` needed.
- `Execute(ctx context.Context, req WithdrawRequest) (*WithdrawResponse, int, error)` method implementing the 13-step orchestration flow from `design.md`.
- `validateWithdrawRequest(req WithdrawRequest) (clientID uuid.UUID, desiredValue decimal.Decimal, ifMatch *int, err error)` private function.
- Command type constant: `"WITHDRAW"`.
- Two-pass lot selection algorithm per `design.md` Lot Selection Algorithm section.
- `if_match` validation between pass 1 and pass 2 per `design.md`.
- Idempotency check via `ProcessedCommandRepository.FindByTypeAndOrderID("WITHDRAW", order_id)` before entity lookups.
- Idempotent replay: deserialize stored `responseSnapshot` into `WithdrawResponse`, return with `200 OK`. No writes executed.
- Race handling: if `ProcessedCommandRepository.Create` returns `ErrDuplicate`, re-read via `FindByTypeAndOrderID` and return snapshot with `200`. If re-read fails, return `409`.
- `ErrConcurrencyConflict` from `PositionRepository.Update` within `UnitOfWork.Do` → return `ConcurrencyConflictError` with `409`.
- Atomic persistence within `UnitOfWork.Do`: update each affected Position, serialize response to JSON, create `ProcessedCommand`.
- Validation short-circuits on first failure.
- Private helpers: `toAffectedPosition(p *domain.Position) AffectedPosition`, `deserializeWithdrawSnapshot(snapshot []byte) (*WithdrawResponse, int, error)`.
- Comprehensive unit tests with mocked port interfaces.

## Out of Scope

- HTTP/API Gateway event parsing (step 002).
- Lambda bootstrap wiring (step 003).
- Structured logging.
- Observability instrumentation.

## Required Reads

- `internal/application/deposit_service.go` — reference pattern for `Execute` signature, validation, idempotency flow, `UnitOfWork.Do` usage, error wrapping, snapshot serialization.
- `internal/application/deposit_service_test.go` — reference pattern for mock definitions and test structure.
- `internal/domain/position.go` — `AvailableValue()`, `UpdateAmount()`, `RowVersion()`, `Amount()`, `UnitPrice()`, field accessors.
- `internal/domain/processed_command.go` — `NewProcessedCommand` signature, `ResponseSnapshot()` accessor.
- `internal/domain/errors.go` — `ValidationError`, `ErrNotFound`, `ErrConcurrencyConflict`, `ErrDuplicate` definitions.
- `internal/ports/position_repository.go` — `PositionRepository` interface, specifically `FindByClientAndInstrument` and `Update`.
- `internal/ports/processed_command_repository.go` — `ProcessedCommandRepository` interface.
- `internal/ports/client_repository.go` — `ClientRepository` interface.
- `internal/ports/unit_of_work.go` — `UnitOfWork` interface.

## Allowed Write Paths

- `internal/application/withdraw_service.go`
- `internal/application/withdraw_service_test.go`

## Forbidden Paths

- `internal/domain/` — domain model is stable and not modified by this feature.
- `internal/ports/` — port interfaces are stable.
- `internal/adapters/` — adapter changes belong to other steps.
- `cmd/` — bootstrap belongs to step 003.

## Known Abstraction Opportunities

- `toAffectedPosition(p *domain.Position) AffectedPosition` helper to map Position to DTO.
- `deserializeWithdrawSnapshot(snapshot []byte) (*WithdrawResponse, int, error)` helper for idempotent replay.
- `replayAfterRace` private method for `ErrDuplicate` race recovery (same pattern as deposit).

## Allowed Abstraction Scope

- Private helpers within `withdraw_service.go` only. No new packages, no new interfaces, no new port definitions.

## Required Tests

All tests use mocked port interfaces defined in the test file. The mocks for `ClientRepository`, `PositionRepository`, `ProcessedCommandRepository`, and `UnitOfWork` follow the same pattern as `deposit_service_test.go`. The `PositionRepository` mock must support `FindByClientAndInstrument` and `Update` function fields. The `UnitOfWork` mock executes the provided function synchronously.

### Happy path
1. Valid withdrawal consuming a single lot fully: returns `200` with one affected position, position amount reduced to zero.
2. Valid withdrawal consuming multiple lots in FIFO order: returns `200` with multiple affected positions in FIFO order.
3. Valid withdrawal partially consuming the last lot: returns `200`, last lot has reduced amount, earlier lots fully consumed.
4. Verify `AffectedPosition` field values match the mutated Position state (amount, total_value, unit_price, timestamps, UUIDs).

### Lot selection edge cases
5. Lots with `AvailableValue() <= 0` are skipped during pass 1 and pass 2.
6. Lots with zero amount (fully depleted) are skipped.
7. FIFO order preserved: oldest `purchased_at` lot consumed first (relies on repository return order).
8. `units_sold` computation: verify `(valueFromLot / lot.UnitPrice()).Round(6)` with half-up rounding produces correct result for a known test case.

### if_match validation
9. `if_match` provided and all affected lots match: withdrawal proceeds normally.
10. `if_match` provided and one affected lot has mismatched `RowVersion()`: returns `409` with `ConcurrencyConflictError`.
11. `if_match` not provided: validation skipped, withdrawal proceeds.

### Insufficient position
12. Total available value across all eligible lots is less than `desired_value`: returns `409` with `InsufficientPositionError`.

### No lots found
13. `FindByClientAndInstrument` returns empty slice: returns `404` with plain error.

### Idempotency — replay
14. When `FindByTypeAndOrderID` returns an existing ProcessedCommand, return its deserialized snapshot with `200`. No `Update` or `Create` calls made.

### Idempotency — concurrent race
15. When `ProcessedCommandRepository.Create` returns `ErrDuplicate`, the service re-reads via `FindByTypeAndOrderID` and returns the snapshot with `200`.
16. When `ProcessedCommandRepository.Create` returns `ErrDuplicate` AND the re-read fails, return `409`.

### Concurrency conflict from Update
17. `PositionRepository.Update` returns `ErrConcurrencyConflict` inside `UnitOfWork.Do`: returns `409` with `ConcurrencyConflictError`.

### Validation failures (all return `422`)
18. Missing `client_id` → `"client_id is required"`.
19. Invalid `client_id` (not a UUID) → `"invalid client_id"`.
20. Missing `product_asset_id` → `"product_asset_id is required"`.
21. Missing `order_id` → `"order_id is required"`.
22. Missing `desired_value` → `"desired_value is required"`.
23. Invalid `desired_value` (not a decimal) → `"invalid desired_value"`.
24. `desired_value` is zero → `"desired_value must be positive"`.
25. `desired_value` is negative → `"desired_value must be positive"`.
26. `if_match` present but not a valid integer → `"invalid if_match"`.

### Entity lookup failures
27. `ClientRepository.FindByID` returns `ErrNotFound` → `422` with `"client not found"`.

### Unexpected errors (return `500`)
28. `ClientRepository.FindByID` returns an unexpected error → `500`.
29. `ProcessedCommandRepository.FindByTypeAndOrderID` returns an unexpected error on initial check → `500`.
30. `PositionRepository.FindByClientAndInstrument` returns an unexpected error → `500`.
31. `UnitOfWork.Do` propagates an unexpected error from `PositionRepository.Update` (not `ErrConcurrencyConflict`) → `500`.

## Coverage Requirement

100% line coverage on `internal/application/withdraw_service.go`.

## Failure Model

- Validation errors → `422` with descriptive message. Explicit, not swallowed.
- `ErrNotFound` from Client → `422`. Treated as input validation failure.
- Empty lots → `404` with `"no positions found"`.
- Insufficient total available → `409` via `InsufficientPositionError`.
- `if_match` mismatch → `409` via `ConcurrencyConflictError`.
- `ErrConcurrencyConflict` from `PositionRepository.Update` → `409` via `ConcurrencyConflictError`.
- `ErrDuplicate` from `ProcessedCommandRepository.Create` → handled by re-read. If re-read fails → `409`.
- Any other error → `500`. Propagated with context via `fmt.Errorf("...: %w", err)`.
- Errors wrapped with `%w` to preserve cause chain for `errors.Is` / `errors.As` inspection.

## Allowed Fallbacks

- `ErrDuplicate` race resolution via re-read is the only recovery path. This is an explicit design requirement, not a hidden fallback.

## Acceptance Criteria

1. `WithdrawService.Execute` implements the full 13-step orchestration flow from `design.md`.
2. All validation rules (1–8 from design.md validation table) are enforced in order, short-circuiting on first failure.
3. Idempotency check happens before entity lookups.
4. Client lookup and lot fetching happen outside `UnitOfWork.Do`.
5. Position updates and ProcessedCommand creation happen inside `UnitOfWork.Do`.
6. Two-pass lot selection: pass 1 is read-only sufficiency check, pass 2 performs mutations only when success is guaranteed.
7. `if_match` validation occurs between pass 1 and pass 2.
8. FIFO order is preserved (relies on repository return order, not service-level sorting).
9. `units_sold = (valueFromLot / lot.UnitPrice()).Round(6)` with half-up rounding.
10. `InsufficientPositionError` and `ConcurrencyConflictError` are typed errors inspectable via `errors.As`.
11. `WithdrawResponse` and `AffectedPosition` JSON tags match the response contract in `design.md`.
12. Decimal fields in `AffectedPosition` serialize as strings.
13. All test scenarios pass.
14. `go build ./...` succeeds.
15. `go vet ./...` produces no warnings on new files.

## Deferred Work

None.

## Escalation Conditions

- If `Position.UpdateAmount` rejects a valid non-negative amount during pass 2, investigate — pass 1 sufficiency check should guarantee non-negative results.
- If shopspring/decimal `Div` then `Round(6)` does not produce half-up rounding by default, investigate and use the appropriate rounding method. The design specifies half-up semantics.
- If `FindByClientAndInstrument` does not return lots ordered by `purchased_at ASC`, the lot selection algorithm will produce incorrect FIFO behavior — verify against the postgres implementation before proceeding.
