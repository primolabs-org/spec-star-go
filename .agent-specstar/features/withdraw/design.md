# Withdraw Command

## Problem Statement

The wallet microservice supports depositing fixed-income position lots but has no way to withdraw value from them. Upstream systems need an HTTP endpoint to reduce position lots for a client by a desired monetary value, consuming lots in FIFO (oldest-first) order with all-or-nothing semantics and safe retry via idempotency.

## Goal

Deliver an HTTP `POST /withdrawals` endpoint on AWS Lambda (API Gateway HTTP API v2) that validates a withdrawal request, selects lots in FIFO order, reduces their amounts by the required units, records idempotency state, and returns the affected positions — all within a single atomic transaction.

## Functional Requirements

1. Accept `POST` requests with JSON body containing: `client_id` (UUID), `product_asset_id` (string — maps to `Asset.InstrumentID()`), `order_id` (string), `desired_value` (decimal string). Optional: `if_match` (string — row_version concurrency token).
2. All four required fields must be present and valid. Reject requests with missing or unparseable fields.
3. Validate `desired_value > 0` (strictly positive).
4. Validate `client_id` references an existing Client via `ClientRepository.FindByID`.
5. Resolve lots via `PositionRepository.FindByClientAndInstrument(ctx, clientID, product_asset_id)`. The `product_asset_id` value is passed directly as the `instrumentID` parameter — no asset lookup is needed.
6. If no lots are returned, reject with 404.
7. **Two-pass lot selection algorithm**:
   - **Pass 1 — Sufficiency check**: Iterate all returned lots. For each lot, compute `available_value = lot.AvailableValue()`. Skip lots where `available_value <= 0`. Sum available values across eligible lots. If total available < `desired_value`, reject with 409 `INSUFFICIENT_POSITION`.
   - **Pass 2 — Mutation**: Iterate eligible lots again in FIFO order. For each lot:
     - `value_from_lot = min(remaining, lot.AvailableValue())`
     - `units_sold = (value_from_lot / lot.UnitPrice()).Round(6)` (shopspring/decimal `Div` then `Round` with half-up semantics)
     - `lot.UpdateAmount(lot.Amount().Sub(units_sold))`
     - Subtract `value_from_lot` from remaining
     - Stop when remaining == 0
8. **if_match validation**: If `if_match` is provided, before pass 2 mutations, verify that every affected lot (lots that will be mutated based on pass 1 results) has `RowVersion()` equal to the parsed `if_match` integer. If any mismatch, reject with 409 `CONCURRENCY_CONFLICT`.
9. **Idempotency**: before executing the withdrawal, check `ProcessedCommandRepository.FindByTypeAndOrderID("WITHDRAW", order_id)`.
   - If found: deserialize `response_snapshot` and return it with 200 OK. No writes executed.
   - If not found: proceed with withdrawal.
10. Persist all lot updates and `ProcessedCommand` atomically within `UnitOfWork.Do`.
11. The `ProcessedCommand.responseSnapshot` stores the JSON-serialized `WithdrawResponse`.
12. If a concurrent request races on the same `order_id`, the unique index `(command_type, order_id)` causes `ErrDuplicate` from `ProcessedCommand.Create`. Handle by re-reading and returning the snapshot with 200 OK.

## Response Contract

### Status Codes

| Scenario | Status | Body |
|---|---|---|
| Successful withdrawal | `200 OK` | WithdrawResponse JSON |
| Idempotent replay (existing `order_id`) | `200 OK` | WithdrawResponse JSON from stored snapshot |
| Validation failure (missing/invalid fields, unknown client) | `422 Unprocessable Entity` | Error JSON |
| No eligible lots for (client_id, instrument_id) | `404 Not Found` | Error JSON |
| Insufficient total available value | `409 Conflict` | Error JSON with `error_code: "INSUFFICIENT_POSITION"` |
| if_match mismatch | `409 Conflict` | Error JSON with `error_code: "CONCURRENCY_CONFLICT"` |
| Optimistic locking failure during Update | `409 Conflict` | Error JSON with `error_code: "CONCURRENCY_CONFLICT"` |
| Idempotency storage conflict (re-read fails) | `409 Conflict` | Error JSON |
| Unexpected server error | `500 Internal Server Error` | Error JSON |

### Request DTO

```json
{
  "client_id": "uuid",
  "product_asset_id": "string",
  "order_id": "string",
  "desired_value": "decimal-string",
  "if_match": "integer-string"
}
```

`desired_value` is transmitted as a decimal string to preserve precision. Parsed with `decimal.NewFromString`. `if_match` is optional; when present, parsed with `strconv.Atoi`.

### Response DTO — WithdrawResponse

```json
{
  "affected_positions": [
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

### Error DTO

```json
{
  "error": "human-readable message",
  "error_code": "INSUFFICIENT_POSITION"
}
```

The `error_code` field is included only for 409 responses with codes `INSUFFICIENT_POSITION` or `CONCURRENCY_CONFLICT`. All other error responses use the plain `{"error": "..."}` shape.

## Validation Rules

All validations produce `422 Unprocessable Entity`. Validation short-circuits on first failure.

| # | Rule | Error message |
|---|---|---|
| 1 | Request body is valid JSON | `invalid request body` |
| 2 | `client_id` is present and valid UUID | `client_id is required` / `invalid client_id` |
| 3 | `product_asset_id` is present and non-empty | `product_asset_id is required` |
| 4 | `order_id` is present and non-empty | `order_id is required` |
| 5 | `desired_value` is present and valid decimal | `desired_value is required` / `invalid desired_value` |
| 6 | `desired_value > 0` | `desired_value must be positive` |
| 7 | `if_match` (if present) is valid integer | `invalid if_match` |
| 8 | Client exists (`FindByID`) | `client not found` |

Rules 1–7 are input parsing/validation in the application layer (before any DB calls).
Rule 8 is an entity lookup in the application layer (before lot selection).

## Lot Selection Algorithm

### Pass 1 — Sufficiency Check (read-only)

```
lots = FindByClientAndInstrument(ctx, clientID, product_asset_id) // FIFO order
if len(lots) == 0 → 404 "no positions found"

totalAvailable = 0
eligibleLots = []
for each lot in lots:
    av = lot.AvailableValue()
    if av > 0:
        totalAvailable += av
        eligibleLots = append(eligibleLots, lot)

if totalAvailable < desired_value → 409 INSUFFICIENT_POSITION
```

### if_match Check (between pass 1 and pass 2)

```
if if_match is provided:
    determine affected lots from eligibleLots (those that will be mutated)
    for each affected lot:
        if lot.RowVersion() != if_match → 409 CONCURRENCY_CONFLICT
```

To determine affected lots without duplicating the withdrawal math, iterate eligible lots and track which ones would be touched by the withdrawal amount. This uses the same FIFO order and `min(remaining, available)` logic as pass 2 but without mutation.

### Pass 2 — Mutation

```
remaining = desired_value
affectedPositions = []
for each lot in eligibleLots:
    if remaining == 0: break
    valueFromLot = min(remaining, lot.AvailableValue())
    unitsSold = (valueFromLot / lot.UnitPrice()).Round(6)
    lot.UpdateAmount(lot.Amount().Sub(unitsSold))
    remaining -= valueFromLot
    affectedPositions = append(affectedPositions, lot)
```

## Idempotency Design

- Command type constant: `"WITHDRAW"`.
- Lookup: `ProcessedCommandRepository.FindByTypeAndOrderID("WITHDRAW", order_id)`.
- If found: return stored `response_snapshot` with `200 OK`. No writes executed.
- If not found: execute withdrawal within `UnitOfWork.Do`:
  1. Update each affected Position via `PositionRepository.Update`.
  2. Serialize `WithdrawResponse` to JSON.
  3. Create `ProcessedCommand` with snapshot via `ProcessedCommandRepository.Create`.
- **Race handling**: if `ProcessedCommand.Create` returns `ErrDuplicate`, re-read via `FindByTypeAndOrderID` and return its snapshot with `200 OK`. If the re-read fails, return `409 Conflict`.
- Idempotency applies regardless of whether the replayed payload matches the original — the `order_id` alone determines idempotency.

## Application Layer Design

A `WithdrawService` in `internal/application/` with a single public method:

```
Execute(ctx context.Context, req WithdrawRequest) (*WithdrawResponse, int, error)
```

Dependencies injected via constructor:
- `ClientRepository`
- `PositionRepository`
- `ProcessedCommandRepository`
- `UnitOfWork`

Note: `AssetRepository` is not needed — `product_asset_id` is passed directly to `FindByClientAndInstrument` as the `instrumentID` string.

### Orchestration Flow

1. Parse and validate input (rules 1–7).
2. Check idempotency. If hit, return early with 200.
3. Lookup Client (rule 8).
4. Find lots via `FindByClientAndInstrument`.
5. If no lots returned, return 404.
6. Pass 1: compute eligible lots and total available.
7. If insufficient, return 409 `INSUFFICIENT_POSITION`.
8. Determine affected lots and validate `if_match` if provided.
9. Pass 2: mutate affected lots.
10. Build `WithdrawResponse` from affected positions.
11. Within `UnitOfWork.Do`: update each affected Position, serialize response, persist ProcessedCommand.
12. Handle `ErrDuplicate` from step 11 as concurrent idempotency race.
13. Handle `ErrConcurrencyConflict` from step 11 as optimistic locking failure → 409 `CONCURRENCY_CONFLICT`.

The Client lookup and lot fetching happen **outside** the transaction to minimize transaction duration and lock scope. Lot mutations (`UpdateAmount`) happen before the transaction but `PositionRepository.Update` is called inside the transaction. If optimistic locking fails (another process modified the lot), the transaction rolls back and `ErrConcurrencyConflict` is returned.

### Validation Function Signature

```go
func validateWithdrawRequest(req WithdrawRequest) (clientID uuid.UUID, desiredValue decimal.Decimal, ifMatch *int, err error)
```

Returns parsed `clientID`, `desiredValue`, and optional `ifMatch` (nil when not provided). `product_asset_id` and `order_id` are validated but passed through as strings.

## HTTP Handler Design

### Inbound Adapter

A `WithdrawHandler` in `internal/adapters/inbound/httphandler/`:
- Uses `events.APIGatewayV2HTTPRequest` / `events.APIGatewayV2HTTPResponse`.
- Accepts an interface for the service (same pattern as `depositExecutor`).
- Parses the event body and delegates to `WithdrawService.Execute`.
- Maps the returned status code and response/error to the API Gateway response.
- Does not contain business logic.

### Handler Routing

The existing `main.go` registers `lambda.Start(handler.Handle)` for a single handler. With two endpoints (`POST /deposits` and `POST /withdrawals`), the Lambda needs a routing layer.

Approach: introduce a thin router function in `cmd/http-lambda/main.go` (or a shared handler) that inspects `req.RequestContext.HTTP.Path` (or `req.RouteKey`) and dispatches to the appropriate handler. The individual handlers (`DepositHandler`, `WithdrawHandler`) retain their `Handle` methods unchanged — they already validate `POST` method.

```go
func route(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
    switch req.RequestContext.HTTP.Path {
    case "/deposits":
        return depositHandler.Handle(ctx, req)
    case "/withdrawals":
        return withdrawHandler.Handle(ctx, req)
    default:
        return notFoundResponse()
    }
}
```

This keeps each handler focused and avoids coupling them to routing concerns.

## Bootstrap Changes

`cmd/http-lambda/main.go` modifications:
1. Instantiate `WithdrawService` with `clients`, `positions`, `processedCommands`, `unitOfWork` (no `assets` needed).
2. Instantiate `WithdrawHandler` with the service.
3. Replace `lambda.Start(handler.Handle)` with `lambda.Start(route)` where `route` dispatches by path.

## Error Mapping

| Domain / Application condition | HTTP Status | Error code |
|---|---|---|
| `ValidationError` (input validation) | `422` | — |
| `ErrNotFound` from Client lookup | `422` (validation: entity not found) | — |
| No lots returned for client+instrument | `404` | — |
| Insufficient total available value | `409` | `INSUFFICIENT_POSITION` |
| `if_match` mismatch (pre-mutation check) | `409` | `CONCURRENCY_CONFLICT` |
| `ErrConcurrencyConflict` from Position Update | `409` | `CONCURRENCY_CONFLICT` |
| `ErrDuplicate` from ProcessedCommand (resolved by re-read) | `200` | — |
| `ErrDuplicate` from ProcessedCommand (re-read fails) | `409` | — |
| Any unexpected error | `500` | — |

The `error_code` field in the response body is populated only for the 409 cases with explicit codes. The handler must distinguish between error responses that carry an `error_code` and those that do not.

### Error Code Propagation

The `WithdrawService.Execute` method must communicate the `error_code` to the handler. Options:
- Return typed errors (e.g., `InsufficientPositionError`, `ConcurrencyConflictError`) that the handler can inspect via `errors.As`.
- Return the error code alongside the status code.

The recommended approach is typed application-level errors that the handler maps to the error DTO shape. This keeps the handler thin while allowing structured error responses.

## Key Design Decisions

1. **Two-pass algorithm for all-or-nothing semantics.** Pass 1 sums total available value across all eligible lots without mutation. If the total is insufficient, the request is rejected immediately — no rollback needed, no partial mutations to undo. Pass 2 only executes when success is guaranteed. This is simpler and safer than a single-pass approach that would need rollback on insufficiency.

2. **`product_asset_id` → `instrumentID` mapping.** The API field `product_asset_id` maps directly to the `instrumentID` string used by `Asset.InstrumentID()` and `PositionRepository.FindByClientAndInstrument`. No `AssetRepository` lookup is needed — the repository already joins through the `assets` table internally. The naming difference is an API-level concern only.

3. **Rounding: `(value / unitPrice).Round(6)` with half-up semantics.** shopspring/decimal's `Div` performs exact decimal division (producing full precision). Calling `.Round(6)` rounds to 6 decimal places. shopspring/decimal's default `Round` method uses banker's rounding (`RoundHalfEven`). To get `ROUND_HALF_UP`, use `decimal.DivRound(value, unitPrice, 6)` or manually `value.Div(unitPrice).RoundBank(6)`. The implementation must verify the correct rounding mode. The specification requires `HALF_UP`, which in shopspring maps to using the `RoundHalfUp` constant with the `StringFixed` approach or custom rounding. Implementation detail: use `value.Div(unitPrice)` then apply rounding to 6 decimal places with half-up semantics via shopspring's available API.

4. **`if_match` applies per-lot.** The `if_match` value is a single integer compared against each affected lot's `RowVersion()`. If any affected lot has a mismatching row_version, the request is rejected with 409 before any mutation. This is a simplification — a more complete design would use per-lot tokens, but the current requirement specifies a single value.

5. **Handler routing via path dispatch.** The existing Lambda registers a single handler. Adding `POST /withdrawals` requires dispatching by `req.RequestContext.HTTP.Path`. A thin router function in `main.go` dispatches to the appropriate handler. Each handler retains its own method validation. The route function returns 404 for unknown paths.

6. **Response snapshot for idempotency.** The snapshot stores the serialized `WithdrawResponse` (list of affected positions with their post-mutation state). On idempotent replay, the snapshot is deserialized directly — no position re-reads needed.

7. **No new domain methods needed.** `Position.AvailableValue()` computes `totalValue - collateralValue - judiciaryCollateralValue`. `Position.UpdateAmount(newAmount)` validates non-negative, sets amount, re-derives totalValue, increments rowVersion, and sets updatedAt. Both already exist.

8. **No new repository methods needed.** `FindByClientAndInstrument` returns lots ordered by `purchased_at ASC` (FIFO). `Update` uses optimistic concurrency via `row_version`. All required methods exist in the current contracts.

9. **Decimal division precision.** shopspring/decimal's `Div` produces results with up to `DivisionPrecision` (default 16) decimal places. Rounding to 6 after division ensures consistent unit quantities. The multiplication `lot.Amount().Sub(unitsSold)` preserves per-lot amount precision.

10. **Successful withdrawal returns 200 (not 201).** Unlike deposit which creates a new resource (201), withdrawal modifies existing resources. 200 is the appropriate status code for both new withdrawals and idempotent replays.

## Non-Functional Requirements

- Sustained throughput: 1,000 req/min.
- Latency: p95 < 200 ms.
- Atomicity: all lot updates + ProcessedCommand creation in single transaction via `UnitOfWork.Do`.
- Optimistic concurrency via `row_version` on each `Position.Update`.
- Minimize database round-trips: idempotency check (1), client lookup (1), lot query (1), transaction with N position updates + 1 command insert (1 transaction, N+1 statements).
- Total: 3 + 1 transaction in the non-idempotent path, 1 in the idempotent path.
- Safe for upstream retries by design.

## Scope

### In Scope

- `POST /withdrawals` endpoint via API Gateway HTTP API v2 → Lambda.
- Application service (`WithdrawService`) with withdrawal orchestration, validation, lot selection, and idempotency.
- Request/response DTOs and JSON serialization.
- HTTP handler (`WithdrawHandler`) as thin inbound adapter.
- Routing changes in `cmd/http-lambda/main.go` to dispatch both deposit and withdrawal paths.
- Error mapping from domain/application to HTTP, including structured `error_code` for 409 responses.
- Tests for application service, HTTP handler, and routing.

### Out of Scope

- Ledger entries, settlement, clearing.
- Pricing recalculation, mark-to-market.
- Partial withdrawal acceptance (all-or-nothing only).
- Collateral or judiciary collateral value modification.
- Read/query endpoints.
- Authorization / authentication.
- Structured logging (separate concern).
- New database migrations (schema already supports this).
- New domain methods or entities.
- New repository methods.
- Asset or Client creation via this endpoint.

## Constraints and Assumptions

- Go 1.25, no ORM, no heavy HTTP frameworks.
- Aurora PostgreSQL with existing schema from `001_initial_schema.sql`.
- Existing `TransactionRunner` (UnitOfWork) is used for atomic writes.
- All repositories already implemented and tested.
- The domain model (`Position`, `ProcessedCommand`, `Client`, `Asset`) is stable and not modified by this feature.
- Repository contracts are not modified — all required methods already exist.
- `FindByClientAndInstrument` returns lots ordered by `purchased_at ASC` (FIFO), confirmed in the existing postgres implementation.
- `Position.UpdateAmount` validates `newAmount >= 0` and returns `ValidationError` if negative — pass 1 sufficiency check guarantees this cannot happen.
- `PositionRepository.Update` uses optimistic concurrency with `row_version` — zero rows affected returns `ErrConcurrencyConflict`.
- shopspring/decimal is used for all monetary arithmetic. Division precision is configurable but defaults to 16 decimal places. Rounding to 6 places with half-up semantics is applied to computed unit quantities.

## Existing Context

The withdraw feature builds on patterns established by the deposit feature:
- `DepositService` in [internal/application/deposit_service.go](internal/application/deposit_service.go) — `Execute(ctx, req) -> (resp, statusCode, error)` pattern.
- `DepositHandler` in [internal/adapters/inbound/httphandler/deposit_handler.go](internal/adapters/inbound/httphandler/deposit_handler.go) — thin adapter with `depositExecutor` interface, `errorResponse` and `jsonResponse` helpers.
- Lambda bootstrap in [cmd/http-lambda/main.go](cmd/http-lambda/main.go) — wires repositories, service, handler, calls `lambda.Start`.
- `Position.AvailableValue()` and `Position.UpdateAmount()` in [internal/domain/position.go](internal/domain/position.go).
- `FindByClientAndInstrument` FIFO query in [internal/adapters/outbound/postgres/position_repository.go](internal/adapters/outbound/postgres/position_repository.go).
- Optimistic concurrency in `PositionRepository.Update` using `row_version` in the WHERE clause.

## Implementation Layout

### New Files

| Path | Purpose |
|---|---|
| `internal/application/withdraw_service.go` | `WithdrawService` struct, `WithdrawRequest` / `WithdrawResponse` / `AffectedPosition` DTOs, `Execute` method, validation, lot selection |
| `internal/application/withdraw_service_test.go` | Unit tests with mocked port interfaces |
| `internal/adapters/inbound/httphandler/withdraw_handler.go` | `WithdrawHandler` struct, `Handle` method mapping API Gateway v2 events |
| `internal/adapters/inbound/httphandler/withdraw_handler_test.go` | Handler mapping and routing tests |

### Modified Files

| Path | Change |
|---|---|
| `cmd/http-lambda/main.go` | Add `WithdrawService` and `WithdrawHandler` instantiation; replace `lambda.Start(handler.Handle)` with path-based router dispatching to both handlers |

### DTO Ownership

- `WithdrawRequest`, `WithdrawResponse`, and `AffectedPosition` live in `internal/application/` — they are the application service contract.
- The HTTP handler deserializes the raw event body into `WithdrawRequest` and serializes `WithdrawResponse` (or error) into the API Gateway response body.
- The response snapshot stored in `ProcessedCommand` is the JSON-serialized `WithdrawResponse`.
- The error DTO with `error_code` is shaped by the handler based on typed errors returned by the service.

## Technical Architecture Details

### Typed Application Errors

The `WithdrawService` defines two typed errors in `withdraw_service.go` for structured error code propagation:

- `InsufficientPositionError` — returned when total available value across eligible lots is less than `desired_value`. The handler maps this to `409` with `error_code: "INSUFFICIENT_POSITION"`.
- `ConcurrencyConflictError` — returned when `if_match` mismatches a lot's `RowVersion()` or when `PositionRepository.Update` returns `ErrConcurrencyConflict`. The handler maps this to `409` with `error_code: "CONCURRENCY_CONFLICT"`.

Both implement the `error` interface with lowercase messages. The handler inspects them via `errors.As` and produces the `{"error": "...", "error_code": "..."}` response shape.

For errors that produce 409 without an error code (e.g., re-read failure after idempotency race), the service returns a plain `fmt.Errorf` with status `409`. The handler uses the existing `errorResponse` helper for these.

### Handler Error Response Strategy

The existing `errorResponse` helper produces `{"error": "..."}`. For 409 responses that carry an `error_code`, the `WithdrawHandler` uses a private `codedErrorResponse` helper that produces `{"error": "...", "error_code": "..."}`. The handler inspects typed errors from `Execute` to decide which shape to use.

The handler receives `(*WithdrawResponse, int, error)` from `Execute`. When `err != nil`:
1. Check `errors.As(err, &InsufficientPositionError{})` → `codedErrorResponse(409, message, "INSUFFICIENT_POSITION")`
2. Check `errors.As(err, &ConcurrencyConflictError{})` → `codedErrorResponse(409, message, "CONCURRENCY_CONFLICT")`
3. Otherwise → `errorResponse(statusCode, message)` (reuse existing helper)

### Shared Response Helpers

The existing `jsonResponse` and `errorResponse` are package-private functions in `deposit_handler.go`. The `WithdrawHandler` file can call them directly since they share the `httphandler` package. No extraction to a separate file is needed.

### WithdrawResponse Snapshot Deserialization

The idempotent replay deserializes the stored snapshot into `WithdrawResponse`. This mirrors the deposit pattern but uses a `WithdrawResponse`-typed deserializer (private function in `withdraw_service.go`).

### Route Function

The route function in `main.go` is a package-level function (not exported) with signature:

```go
func route(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)
```

It captures `depositHandler` and `withdrawHandler` via closure. The 404 default returns `{"error": "not found"}` with `Content-Type: application/json`.

### Error Wrapping Convention

All errors propagated from repository calls are wrapped with `fmt.Errorf("operation context: %w", err)` to preserve cause chains, consistent with the deposit service pattern.

## Technical Debt

### Discovered In-Scope Issues (Deferred)

1. **Structured logging not yet wired.** The design.md explicitly defers structured logging. The withdraw service follows the same pattern as the deposit service (no logging in the application layer). Structured logging will be a separate feature.

2. **Observability instrumentation not yet wired.** The design.md explicitly defers traces and metrics. The withdraw service follows the same pattern as the deposit service.

3. **`jsonMarshal` test seam in handler.** The deposit handler uses `var jsonMarshal = json.Marshal` as a test seam. The withdraw handler should reference the same package-level variable rather than declaring a second one. This is already achievable since both handlers are in the same package.

## Open Questions

None — requirements are sufficiently specified for architecture and implementation.

## Success Criteria

1. A `POST /withdrawals` with valid payload and sufficient available value returns 200 with the list of affected positions.
2. Lots are consumed in FIFO order (oldest `purchased_at` first).
3. A repeated `POST` with the same `order_id` returns the stored response with 200.
4. A request with `desired_value` exceeding total available value returns 409 with `INSUFFICIENT_POSITION`.
5. A request with `if_match` mismatching any affected lot's `row_version` returns 409 with `CONCURRENCY_CONFLICT`.
6. An optimistic locking failure during lot update returns 409 with `CONCURRENCY_CONFLICT`.
7. Invalid requests (missing fields, non-positive desired_value, unknown client, invalid if_match) return 422.
8. Request for a client+instrument combination with no lots returns 404.
9. All lot updates and ProcessedCommand are persisted atomically.
10. Concurrent duplicate `order_id` requests do not produce duplicate withdrawals.
11. The Lambda handler contains no business logic.
12. Existing deposit functionality is unaffected by routing changes.
13. All changed lines have test coverage.
