# Withdraw Command

## Metadata
- Feature: withdraw
- Status: draft
- Owner: SpecStar
- Last Updated: 2026-04-19
- Source Request: Add a withdraw command endpoint that debits a desired BRL value from a client's fixed-income position using FIFO lot selection.

## Problem Statement

Clients hold fixed-income positions as one or more deposit lots. There is currently no mechanism for a client to withdraw a desired BRL value from those positions. The system must determine which lots to debit and by how many units, following FIFO order and respecting collateral constraints, without requiring the client to specify individual lots.

## Goal

Deliver an HTTP `POST /withdrawals` endpoint on the existing AWS Lambda that validates a withdrawal request, selects eligible lots in FIFO order, computes unit reductions to satisfy the desired BRL value, persists all lot updates atomically with idempotency state, and returns the affected lot states.

## Functional Requirements

### Endpoint

- FR-1: Expose `POST /withdrawals` on the existing HTTP Lambda (API Gateway HTTP API v2).
- FR-2: Request payload fields:
  - `client_id` (string, required) — UUID of the client.
  - `instrument_id` (string, required) — instrument identifier matching `Asset.InstrumentID()`.
  - `order_id` (string, required) — external identifier for idempotency.
  - `desired_value` (string, required) — decimal string representing the BRL value to withdraw.

### Validation

FR-3: Reject with `422 Unprocessable Entity` when any validation rule fails. Validation short-circuits on first failure.

| # | Rule | Error message |
|---|---|---|
| 1 | Request body is valid JSON | `invalid request body` |
| 2 | `client_id` is present and valid UUID | `client_id is required` / `invalid client_id` |
| 3 | `instrument_id` is present and non-empty | `instrument_id is required` |
| 4 | `order_id` is present and non-empty | `order_id is required` |
| 5 | `desired_value` is present and valid decimal | `desired_value is required` / `invalid desired_value` |
| 6 | `desired_value > 0` | `desired_value must be positive` |
| 7 | Client exists (`FindByID`) | `client not found` |

Rules 1–6 are input parsing/validation in the application layer (before any DB calls).
Rule 7 is an entity lookup in the application layer (before lot selection).

### Lot Selection (FIFO)

- FR-4: Retrieve all positions for `(client_id, instrument_id)` using `PositionRepository.FindByClientAndInstrument`, which returns positions ordered by `purchased_at ASC`.
- FR-5: If no positions are returned, respond `404 Not Found`.
- FR-6: Only lots with `AvailableValue() > 0` are eligible for withdrawal.

### Pre-validation (All-or-Nothing)

- FR-7: Before modifying any lots, sum `AvailableValue()` across all eligible lots. If the total is less than `desired_value`, respond `409 Conflict` with error code `INSUFFICIENT_POSITION`. No lots are modified.

### Withdrawal Computation

- FR-8: For each eligible lot in FIFO order, while `remaining_desired_value > 0`:
  1. `value_from_lot = min(remaining_desired_value, lot.AvailableValue())`
  2. `units_sold = (value_from_lot / lot.UnitPrice()).Round(6)` — round half away from zero (shopspring/decimal `Round(6)` behavior, matches HALF_UP for positive values).
  3. `actual_value_consumed = units_sold.Mul(lot.UnitPrice())`
  4. `remaining_desired_value -= actual_value_consumed`
  5. Call `lot.UpdateAmount(lot.Amount().Sub(units_sold))`.
- FR-9: All lot updates happen within a single database transaction via `UnitOfWork.Do`.
- FR-10: Each affected lot is persisted via `PositionRepository.Update`, which enforces optimistic concurrency via `row_version`.

### Concurrency

- FR-11: If any `PositionRepository.Update` call fails with `ErrConcurrencyConflict`, the transaction rolls back and the service returns `409 Conflict` with error code `CONCURRENCY_CONFLICT`.

### Idempotency

- FR-12: Command type constant: `"WITHDRAW"`.
- FR-13: Before executing, check `ProcessedCommandRepository.FindByTypeAndOrderID("WITHDRAW", order_id)`. If found, return `200 OK` with the stored response snapshot.
- FR-14: On successful withdrawal, persist a `ProcessedCommand` with the serialized response snapshot inside the same transaction.
- FR-15: Handle duplicate-key race on `ProcessedCommand` creation by re-reading the processed command and returning its snapshot (same pattern as `DepositService.replayAfterRace`).

### Response Contract

#### Status Codes

| Scenario | Status | Body |
|---|---|---|
| Successful withdrawal | `200 OK` | Positions JSON array |
| Idempotent replay (existing `order_id`) | `200 OK` | Positions JSON array from stored snapshot |
| Insufficient available value | `409 Conflict` | Error JSON with `code` |
| Concurrency conflict on lot update | `409 Conflict` | Error JSON with `code` |
| No lots found for (client_id, instrument_id) | `404 Not Found` | Error JSON |
| Validation failure | `422 Unprocessable Entity` | Error JSON |
| Unexpected server error | `500 Internal Server Error` | Error JSON |

#### Response DTO — Successful Withdrawal

```json
{
  "positions": [
    {
      "position_id": "uuid",
      "client_id": "uuid",
      "asset_id": "uuid",
      "amount": "decimal-string",
      "unit_price": "decimal-string",
      "total_value": "decimal-string",
      "collateral_value": "decimal-string",
      "judiciary_collateral_value": "decimal-string",
      "purchased_at": "RFC3339",
      "created_at": "RFC3339",
      "updated_at": "RFC3339"
    }
  ]
}
```

#### Error DTO

```json
{
  "error": "human-readable message",
  "code": "INSUFFICIENT_POSITION"
}
```

The `code` field is present only on `409 Conflict` responses where a machine-readable distinction is needed. Other error responses use only `{"error": "message"}` consistent with the deposit pattern.

### Request DTO

```json
{
  "client_id": "uuid-string",
  "instrument_id": "string",
  "order_id": "string",
  "desired_value": "decimal-string"
}
```

## Non-Functional Requirements

- NFR-1: p95 latency < 200ms.
- NFR-2: Throughput: 1,000 req/min sustained.
- NFR-3: Atomic multi-lot withdrawal within a single database transaction.
- NFR-4: Optimistic concurrency enforcement per lot via `row_version`.
- NFR-5: Decimal string serialization for all monetary and quantity values to preserve precision.
- NFR-6: Minimize transaction duration — client lookup outside transaction, position reads and updates inside.

## Scope

### In Scope

- `WithdrawService` in `internal/application/` — orchestrates validation, lot selection, computation, and transactional persistence.
- `WithdrawHandler` in `internal/adapters/inbound/httphandler/` — maps API Gateway v2 events to the service.
- Lambda routing in `cmd/http-lambda/main.go` — dispatch by request path to deposit or withdraw handler.
- Tests for all new code.

### Out of Scope

- `if_match` request field. The existing per-row `row_version` optimistic locking in `PositionRepository.Update` already provides concurrency safety. A client-facing `if_match` header adds multi-lot complexity with limited incremental value. Defer to a future feature if needed.
- Partial withdrawal acceptance (if insufficient, the entire request fails).
- Ledger entries, settlement, or clearing records.
- Pricing recalculation or MTM updates.
- Collateral or judiciary value modification (only `amount` and derived fields change).
- Read model / query endpoints.
- Authorization / authentication.
- New domain entities, port interfaces, or schema migrations.

## Constraints and Assumptions

- Go 1.25+ (from `go.mod`), AWS Lambda with API Gateway HTTP API v2.
- Hexagonal architecture: domain and ports have zero infrastructure dependencies.
- `PositionRepository.FindByClientAndInstrument` already returns positions ordered by `purchased_at ASC` and works with transaction context via `executorFromContext`.
- `PositionRepository.Update` already enforces `row_version` optimistic concurrency.
- `shopspring/decimal` `Round(6)` uses round-half-away-from-zero for positive values, matching HALF_UP semantics.
- The codebase uses `instrument_id` on Asset (not `product_asset_id`). The request field is `instrument_id`.
- No new dependencies needed — all required libraries are already in `go.mod`.
- `events.APIGatewayV2HTTPRequest.RequestContext.HTTP.Path` provides the request path for routing.

## Existing Context

### Domain Layer (`internal/domain/`)
- `Position` has `AvailableValue()`, `UpdateAmount(newAmount)`, `RowVersion()`, `UnitPrice()`, `Amount()`.
- `errors.go` provides `ErrNotFound`, `ErrConcurrencyConflict`, `ErrDuplicate`, `ValidationError`.
- `ProcessedCommand` supports storing command type, order ID, client ID, and response snapshot.

### Ports (`internal/ports/`)
- `PositionRepository.FindByClientAndInstrument(ctx, clientID, instrumentID)` returns `[]*domain.Position` ordered by `purchased_at ASC`.
- `PositionRepository.Update(ctx, position)` enforces optimistic concurrency via `row_version`.
- `ClientRepository.FindByID(ctx, clientID)`.
- `ProcessedCommandRepository.FindByTypeAndOrderID(ctx, commandType, orderID)` and `Create(ctx, cmd)`.
- `UnitOfWork.Do(ctx, fn)`.

### Application Layer (`internal/application/`)
- `DepositService` is the reference pattern: validates input → checks idempotency → entity lookups → executes within `UnitOfWork` → handles race replay.

### HTTP Handler (`internal/adapters/inbound/httphandler/`)
- `DepositHandler` maps API Gateway v2 events, checks HTTP method, unmarshals body, calls service, maps response.
- `jsonResponse` and `errorResponse` are package-level functions, reusable by `WithdrawHandler`.

### Bootstrap (`cmd/http-lambda/main.go`)
- Currently registers only `DepositHandler` and starts Lambda with `handler.Handle`.

## Technical Approach

### Application Service

`WithdrawService` in `internal/application/withdraw_service.go` following the `DepositService` pattern:

```
Execute(ctx, WithdrawRequest) -> (*WithdrawResponse, int, error)
```

Dependencies (constructor-injected):
- `ClientRepository`
- `PositionRepository`
- `ProcessedCommandRepository`
- `UnitOfWork`

Orchestration:
1. Parse and validate input (rules 1–6).
2. Check idempotency. If hit, return early with 200.
3. Lookup Client (rule 7). Outside transaction.
4. Inside `UnitOfWork.Do`:
   a. `FindByClientAndInstrument(txCtx, clientID, instrumentID)`.
   b. If empty, return `ErrNotFound`-based error (mapped to 404 outside tx).
   c. Filter eligible lots (`AvailableValue() > 0`).
   d. Sum available values. If < `desired_value`, return domain-level error (mapped to 409 outside tx).
   e. FIFO loop: compute `units_sold`, call `UpdateAmount`, track `actual_value_consumed`.
   f. `PositionRepository.Update` for each affected lot.
   g. Serialize response, create `ProcessedCommand`.
5. Handle `ErrDuplicate` as race replay.
6. Handle `ErrConcurrencyConflict` as 409.

### Error Mapping

| Condition | HTTP Status |
|---|---|
| Validation failure (rules 1–6) | 422 |
| Client not found | 422 |
| No positions found | 404 |
| Insufficient available value | 409 with `INSUFFICIENT_POSITION` |
| Concurrency conflict | 409 with `CONCURRENCY_CONFLICT` |
| Duplicate ProcessedCommand (resolved by re-read) | 200 |
| Duplicate ProcessedCommand (re-read fails) | 409 |
| Unexpected error | 500 |

To distinguish 409 sub-cases from generic errors at the handler level, the service returns typed/sentinel errors that the handler maps to the appropriate code.

### HTTP Handler

`WithdrawHandler` in `internal/adapters/inbound/httphandler/withdraw_handler.go`:
- Accepts `WithdrawService` as constructor dependency (via interface).
- Checks `req.RequestContext.HTTP.Method == POST`.
- Unmarshals body into `WithdrawRequest`.
- Calls `service.Execute`.
- Maps status code and response/error to API Gateway response.
- Reuses `jsonResponse` and `errorResponse`.

### Lambda Routing

A top-level router in `cmd/http-lambda/main.go`:
- Dispatch by `req.RequestContext.HTTP.Path`:
  - `/deposits` → `depositHandler.Handle`
  - `/withdrawals` → `withdrawHandler.Handle`
  - Default → `404 Not Found`
- Simple `switch` statement, no framework.

### Rounding Residual Handling

The withdrawal loop tracks `remaining_desired_value` using actual consumed values (accounting for rounding) rather than the planned `value_from_lot`. This means:
- `units_sold = (value_from_lot / lot.UnitPrice()).Round(6)`
- `actual_value_consumed = units_sold.Mul(lot.UnitPrice())`
- `remaining_desired_value = remaining_desired_value.Sub(actual_value_consumed)`

This ensures rounding residuals carry forward and the last lot absorbs any cumulative rounding difference. Because the pre-validation confirmed total available >= desired_value, the loop always has sufficient lots to absorb residuals.

### Transaction Scope

Client lookup happens outside the transaction (read-only, consistent with deposit pattern). Position reads, updates, and processed command creation happen inside `UnitOfWork.Do` for atomicity.

Reading positions inside the transaction ensures the FIFO order and available values are consistent with the subsequent updates. If a concurrent modification happens between the client lookup and the transaction start, the optimistic locking in `Update` catches it.

## Affected Components

| Component | Change |
|---|---|
| `internal/application/withdraw_service.go` | New file — `WithdrawService`, DTOs, validation |
| `internal/application/withdraw_service_test.go` | New file — unit tests |
| `internal/adapters/inbound/httphandler/withdraw_handler.go` | New file — `WithdrawHandler` |
| `internal/adapters/inbound/httphandler/withdraw_handler_test.go` | New file — unit tests |
| `cmd/http-lambda/main.go` | Add path routing, wire withdraw dependencies |

## Contracts and Data Shape Impact

- New `WithdrawRequest` struct with `ClientID`, `InstrumentID`, `OrderID`, `DesiredValue` string fields.
- New `WithdrawResponse` struct wrapping a `[]PositionDTO` slice.
- `PositionDTO` has the same fields as `DepositResponse`. Consider extracting a shared `PositionDTO` if the field set is identical, or keep separate if the coupling cost outweighs the deduplication benefit.
- No changes to existing request/response contracts.
- No changes to port interfaces.

## State / Persistence Impact

- **Position rows**: `amount`, `total_value`, `row_version`, `updated_at` updated for affected lots.
- **ProcessedCommand rows**: new `WITHDRAW` command type rows.
- No schema changes required.

## Failure Model

Default: fail-fast. No silent fallbacks.

| Condition | Behavior |
|---|---|
| Invalid input | Reject immediately with 422 |
| Client not found | Reject with 422 |
| No positions | Reject with 404 |
| Insufficient value | Reject with 409 before any modification |
| Concurrency conflict | Transaction rollback, reject with 409 |
| `UpdateAmount` returns error | Propagate — should not happen if pre-validation is correct, but the error must still be checked |
| `json.Marshal` fails | Propagate as 500 |
| Infrastructure failure | Propagate as 500 |

## Testing and Validation Strategy

### Application Layer (`withdraw_service_test.go`)

Unit tests with mock repositories:
- Successful single-lot withdrawal with exact value.
- Successful multi-lot FIFO withdrawal.
- Desired value exactly equals total available across all lots.
- Lot with zero available value is skipped.
- `INSUFFICIENT_POSITION` rejection.
- No positions found (404).
- Concurrency conflict propagation from `Update`.
- Idempotent replay (existing processed command).
- Race condition replay (duplicate on create, successful re-read).
- Race condition replay (duplicate on create, failed re-read → 409).
- All validation error paths (missing/invalid fields, client not found).
- Rounding behavior verification with known decimal values.

### HTTP Handler (`withdraw_handler_test.go`)

Unit tests with mock service:
- Successful request → 200 with JSON body.
- Non-POST method → 405.
- Invalid JSON body → 422.
- Service returns error → correct status code forwarded.

### Coverage

100% on changed executable lines.

## Execution Notes

- `jsonResponse` and `errorResponse` in `deposit_handler.go` are package-level (unexported) functions. They are directly reusable by `withdraw_handler.go` since both files are in the same package.
- The `DepositResponse` DTO field set is identical to each position entry in `WithdrawResponse`. Whether to extract a shared type is a step-level decision.
- The router in `main.go` is a simple `switch` on path. No handler registry or route table.
- Position reads must happen inside the transaction to guarantee consistency between read and update.
- The service needs typed errors for `INSUFFICIENT_POSITION` and `CONCURRENCY_CONFLICT` that the handler can distinguish to set the `code` field in the 409 response.
- `ErrConcurrencyConflict` already exists in `domain/errors.go`. A new sentinel or typed error is needed for `INSUFFICIENT_POSITION` — either a new domain error or an application-level typed error.

## Open Questions

None. All design questions have been resolved.

## Success Criteria

1. `POST /withdrawals` with valid input produces `200 OK` with updated position states.
2. Multi-lot FIFO selection distributes the withdrawal across lots oldest-first.
3. Total available < desired_value is rejected atomically with `409 INSUFFICIENT_POSITION` and no lot modifications.
4. Optimistic concurrency conflicts surface as `409 CONCURRENCY_CONFLICT`.
5. Idempotent replays return `200 OK` with the original response snapshot.
6. All lot updates and processed command creation happen in a single transaction.
7. Test coverage is 100% on changed lines and above 90% on the codebase.
8. No new library dependencies added.
9. Existing deposit endpoint and tests continue to work unchanged.
