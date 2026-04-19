# Step 002 - Implement WithdrawService

## Metadata
- Feature: withdraw
- Step: step-002
- Status: pending
- Depends On: step-001
- Last Updated: 2026-04-19

## Objective

Implement `WithdrawService` in the application layer with full withdrawal orchestration: input validation, idempotency check, client lookup, transactional FIFO lot selection, withdrawal computation, position updates, processed command creation, and race replay handling. Deliver all unit tests.

## In Scope

- New `internal/application/withdraw_service.go` containing:
  - `WithdrawRequest` DTO with `ClientID`, `InstrumentID`, `OrderID`, `DesiredValue` string fields.
  - `WithdrawResponse` DTO wrapping `[]PositionDTO`.
  - `PositionDTO` with fields matching `DepositResponse` (position_id, client_id, asset_id, amount, unit_price, total_value, collateral_value, judiciary_collateral_value, purchased_at, created_at, updated_at).
  - `WithdrawService` struct with constructor `NewWithdrawService` accepting `ClientRepository`, `PositionRepository`, `ProcessedCommandRepository`, `UnitOfWork`.
  - `Execute(ctx, WithdrawRequest) (*WithdrawResponse, int, error)` method implementing the full orchestration.
  - `validateWithdrawRequest` input validation function (short-circuit on first failure).
  - Private helpers: response mapping, snapshot serialization/deserialization, race replay.
- New `internal/application/withdraw_service_test.go` with all unit tests.
- Command type constant: `"WITHDRAW"`.

### Orchestration Flow

1. Validate input (rules 1–6). Return `(nil, 422, err)` on failure.
2. Check idempotency via `ProcessedCommandRepository.FindByTypeAndOrderID("WITHDRAW", orderID)`. If found, deserialize and return `(response, 200, nil)`.
3. Lookup client via `ClientRepository.FindByID`. If `ErrNotFound`, return `(nil, 422, "client not found")`. Other errors → `(nil, 500, err)`.
4. Inside `UnitOfWork.Do`:
   a. `PositionRepository.FindByClientAndInstrument(txCtx, clientID, instrumentID)`.
   b. If empty → return error wrapping `domain.ErrNotFound` (handler maps to 404).
   c. Filter lots with `AvailableValue() > 0`.
   d. Sum available. If < `desiredValue` → return error wrapping `domain.ErrInsufficientPosition`.
   e. FIFO loop: compute `units_sold`, `actual_value_consumed`, call `lot.UpdateAmount`, track remaining.
   f. `PositionRepository.Update` for each affected lot. Propagate errors (including `ErrConcurrencyConflict`).
   g. Build response DTO, serialize snapshot, create `ProcessedCommand`.
5. After `UnitOfWork.Do`:
   - If `ErrDuplicate` → replay via re-read (same pattern as `DepositService.replayAfterRace`).
   - If `ErrConcurrencyConflict` → return `(nil, 409, err)` with wrapped sentinel.
   - If `ErrNotFound` (from empty positions) → return `(nil, 404, err)`.
   - If `ErrInsufficientPosition` → return `(nil, 409, err)` with wrapped sentinel.
   - Other errors → return `(nil, 500, err)`.

### Error Propagation from Transaction

Errors returned from the `UnitOfWork.Do` callback must wrap domain sentinels with `%w` so the post-transaction error classification using `errors.Is` works correctly. The service maps these to the appropriate HTTP status code before returning to the handler.

### Validation Rules (short-circuit)

1. `client_id` present → `"client_id is required"`, valid UUID → `"invalid client_id"`.
2. `instrument_id` present and non-empty → `"instrument_id is required"`.
3. `order_id` present and non-empty → `"order_id is required"`.
4. `desired_value` present → `"desired_value is required"`, valid decimal → `"invalid desired_value"`, > 0 → `"desired_value must be positive"`.

Return type: `(uuid.UUID, decimal.Decimal, error)` — parsed `clientID` and `desiredValue`. `instrumentID` and `orderID` are used as strings.

## Out of Scope

- HTTP handler code.
- Lambda routing.
- Extracting a shared `PositionDTO` type across deposit and withdraw (keep separate to avoid coupling).
- Any changes to domain, ports, or outbound adapters.

## Required Reads

- `internal/application/deposit_service.go` — reference pattern for orchestration, validation, idempotency, race replay, snapshot serialization.
- `internal/application/deposit_service_test.go` — reference pattern for mock setup, test structure, assertion style.
- `internal/domain/position.go` — `AvailableValue()`, `UpdateAmount()`, `Amount()`, `UnitPrice()`, accessor methods.
- `internal/domain/errors.go` — sentinel errors (`ErrNotFound`, `ErrConcurrencyConflict`, `ErrDuplicate`, `ErrInsufficientPosition`).
- `internal/ports/position_repository.go` — `FindByClientAndInstrument`, `Update` signatures.
- `internal/ports/client_repository.go` — `FindByID` signature.
- `internal/ports/processed_command_repository.go` — `FindByTypeAndOrderID`, `Create` signatures.
- `internal/ports/unit_of_work.go` — `Do` signature.

## Allowed Write Paths

- `internal/application/withdraw_service.go`
- `internal/application/withdraw_service_test.go`

## Forbidden Paths

- `internal/domain/`
- `internal/ports/`
- `internal/adapters/`
- `cmd/`

## Known Abstraction Opportunities

- `PositionDTO` field set is identical to `DepositResponse`. A shared `PositionDTO` type could eliminate duplication. Defer unless step instructions explicitly allow extraction.

## Allowed Abstraction Scope

- Private helpers within `withdraw_service.go` for response mapping, snapshot handling, and validation.

## Required Tests

All tests use mock repositories and `mockUnitOfWork` following existing patterns in `deposit_service_test.go`.

1. **Successful single-lot withdrawal**: one lot with sufficient value, verify response contains one PositionDTO with updated amount, status 200.
2. **Successful multi-lot FIFO withdrawal**: multiple lots, verify lots are consumed oldest-first, response contains all affected lots, status 200.
3. **Desired value equals total available**: all lots fully consumed (amounts reach collateral floor), status 200.
4. **Lot with zero available value skipped**: include a lot with `AvailableValue() == 0` among eligible lots, verify it is not in the response, status 200.
5. **INSUFFICIENT_POSITION rejection**: total available < desired_value, verify status 409, verify `errors.Is(err, domain.ErrInsufficientPosition)`.
6. **No positions found**: `FindByClientAndInstrument` returns empty slice, verify status 404.
7. **Concurrency conflict from Update**: mock `Update` returning `ErrConcurrencyConflict`, verify status 409, verify `errors.Is(err, domain.ErrConcurrencyConflict)`.
8. **Idempotent replay**: mock `FindByTypeAndOrderID` returning an existing command, verify status 200, verify deserialized snapshot returned.
9. **Race replay (success)**: `UnitOfWork.Do` returns `ErrDuplicate`, re-read succeeds, verify status 200.
10. **Race replay (re-read fails)**: `UnitOfWork.Do` returns `ErrDuplicate`, re-read fails, verify status 409.
11. **Validation: client_id missing** → 422, `"client_id is required"`.
12. **Validation: client_id invalid UUID** → 422, `"invalid client_id"`.
13. **Validation: instrument_id missing** → 422, `"instrument_id is required"`.
14. **Validation: order_id missing** → 422, `"order_id is required"`.
15. **Validation: desired_value missing** → 422, `"desired_value is required"`.
16. **Validation: desired_value invalid decimal** → 422, `"invalid desired_value"`.
17. **Validation: desired_value zero or negative** → 422, `"desired_value must be positive"`.
18. **Validation: client not found** → 422, `"client not found"`.
19. **Rounding verification**: use known decimal values where `value / unitPrice` produces a repeating decimal, verify `Round(6)` is applied and `actual_value_consumed` accounts for rounding, remaining is correctly tracked.

## Coverage Requirement

100% on all lines in `withdraw_service.go`.

## Failure Model

- Invalid input → 422, no side effects.
- Client not found → 422, no side effects.
- No positions → 404, no side effects.
- Insufficient value → 409, no side effects (pre-validation before any mutation).
- Concurrency conflict → 409, transaction rolled back by `UnitOfWork`.
- `UpdateAmount` validation error → propagate as 500 (should not happen after pre-validation, but error must not be discarded).
- Snapshot marshal/unmarshal failure → 500.
- Infrastructure failure → 500.
- Duplicate processed command → race replay.

## Allowed Fallbacks

None. All failures must be explicit.

## Acceptance Criteria

1. `WithdrawService.Execute` validates input, checks idempotency, performs FIFO lot selection, computes unit reductions, persists atomically, and returns affected positions.
2. Errors from domain sentinels (`ErrInsufficientPosition`, `ErrConcurrencyConflict`, `ErrNotFound`, `ErrDuplicate`) are wrapped with `%w` and classifiable at the caller via `errors.Is`.
3. The FIFO loop correctly computes `units_sold = (value_from_lot / unitPrice).Round(6)` and tracks `actual_value_consumed` to handle rounding residuals.
4. All 19 test cases pass.
5. 100% coverage on `withdraw_service.go`.
6. No changes to existing files.

## Deferred Work

- Shared `PositionDTO` extraction across deposit and withdraw responses.

## Escalation Conditions

- If `FindByClientAndInstrument` does not return positions in `purchased_at ASC` order, escalate — the FIFO guarantee depends on repository ordering.
- If `shopspring/decimal.Round(6)` behavior differs from HALF_UP for positive values, escalate — rounding correctness is critical.
