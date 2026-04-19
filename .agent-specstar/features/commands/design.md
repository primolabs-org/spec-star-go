# Deposit Command

## Problem Statement

The wallet microservice has a domain model and persistence layer but no way to accept deposit commands. Upstream systems need an HTTP endpoint to create fixed-income position lots for clients, with safe retry semantics backed by idempotency.

## Goal

Deliver an HTTP `POST /deposits` endpoint on AWS Lambda (API Gateway HTTP API v2) that validates a deposit request, creates a new Position lot, records idempotency state, and returns the resulting position — all within a single atomic transaction.

## Functional Requirements

1. Accept `POST` requests with JSON body containing: `client_id` (UUID), `asset_id` (UUID), `order_id` (string), `amount` (decimal string), `unit_price` (decimal string).
2. All five fields are required. Reject requests with missing or unparseable fields.
3. Validate `amount > 0` and `unit_price > 0` (strictly positive) at the application layer before calling `NewPosition`.
4. Validate `client_id` references an existing Client via `ClientRepository.FindByID`.
5. Validate `asset_id` references an existing Asset via `AssetRepository.FindByID`.
6. Validate the referenced Asset has a supported `ProductType` via `ValidateProductType`.
7. Compute `total_value = amount × unit_price` (delegated to `NewPosition`).
8. Create Position with `purchased_at = time.Now().UTC()`, `collateral_value = 0`, `judiciary_collateral_value = 0` (delegated to `NewPosition`).
9. **Idempotency**: before executing the deposit, check `ProcessedCommandRepository.FindByTypeAndOrderID("DEPOSIT", order_id)`.
   - If found: deserialize `response_snapshot` and return it. No new writes.
   - If not found: proceed with deposit creation.
10. Persist `Position` and `ProcessedCommand` atomically within `UnitOfWork.Do`.
11. The `ProcessedCommand.responseSnapshot` stores the JSON-serialized position response DTO.
12. If a concurrent request races on the same `order_id`, the unique index `(command_type, order_id)` causes an `ErrDuplicate` from `ProcessedCommand.Create`. The service must handle this by re-reading the existing `ProcessedCommand` and returning its snapshot.

## Response Contract

### Status Codes

| Scenario | Status | Body |
|---|---|---|
| New deposit created | `201 Created` | Position JSON |
| Idempotent replay (existing `order_id`) | `200 OK` | Position JSON from stored snapshot |
| Validation failure (missing/invalid fields, unknown client, unknown asset, unsupported product type, non-positive amounts) | `422 Unprocessable Entity` | Error JSON |
| Idempotency storage conflict that cannot be resolved by re-read | `409 Conflict` | Error JSON |
| Unexpected server error | `500 Internal Server Error` | Error JSON |

### Response DTO — Position

```
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
```

### Error DTO

```
{
  "error": "human-readable message"
}
```

## Request DTO

```
{
  "client_id": "uuid",
  "asset_id": "uuid",
  "order_id": "string",
  "amount": "decimal-string",
  "unit_price": "decimal-string"
}
```

`amount` and `unit_price` are transmitted as decimal strings to preserve precision. Parsed with `decimal.NewFromString`.

## Validation Rules

All validations produce `422 Unprocessable Entity`. Validation short-circuits on first failure.

| # | Rule | Error message |
|---|---|---|
| 1 | Request body is valid JSON | `invalid request body` |
| 2 | `client_id` is present and valid UUID | `client_id is required` / `invalid client_id` |
| 3 | `asset_id` is present and valid UUID | `asset_id is required` / `invalid asset_id` |
| 4 | `order_id` is present and non-empty | `order_id is required` |
| 5 | `amount` is present and valid decimal | `amount is required` / `invalid amount` |
| 6 | `unit_price` is present and valid decimal | `unit_price is required` / `invalid unit_price` |
| 7 | `amount > 0` | `amount must be positive` |
| 8 | `unit_price > 0` | `unit_price must be positive` |
| 9 | Client exists (`FindByID`) | `client not found` |
| 10 | Asset exists (`FindByID`) | `asset not found` |
| 11 | Asset has supported product type (`ValidateProductType`) | `unsupported product type` |

Rules 1–8 are input parsing/validation in the application layer (before any DB calls).
Rules 9–11 are entity lookups in the application layer (before position creation).

## Idempotency Design

- Command type constant: `"DEPOSIT"`.
- Lookup: `ProcessedCommandRepository.FindByTypeAndOrderID("DEPOSIT", order_id)`.
- If found: return stored `response_snapshot` with `200 OK`. No writes executed.
- If not found: execute deposit within `UnitOfWork.Do`:
  1. Create `Position` via `PositionRepository.Create`.
  2. Serialize position response DTO to JSON.
  3. Create `ProcessedCommand` with snapshot via `ProcessedCommandRepository.Create`.
- **Race handling**: if `ProcessedCommand.Create` returns `ErrDuplicate` (concurrent insert won), re-read via `FindByTypeAndOrderID` and return the snapshot with `200 OK`. If the re-read itself fails, return `409 Conflict`.
- Idempotency applies regardless of whether the replayed payload matches the original — the `order_id` alone determines idempotency.

## Application Layer Design

A `DepositService` (or `DepositHandler` — naming follows codebase convention once established) in `internal/application/` (or `internal/usecase/`) with a single public method:

```
Execute(ctx, request) -> (response, statusCode, error)
```

Dependencies injected via constructor:
- `ClientRepository`
- `AssetRepository`
- `PositionRepository`
- `ProcessedCommandRepository`
- `UnitOfWork`

Orchestration flow:
1. Parse and validate input (rules 1–8).
2. Check idempotency. If hit, return early.
3. Lookup Client (rule 9).
4. Lookup Asset (rules 10–11).
5. Create Position via domain constructor.
6. Within `UnitOfWork.Do`: persist Position, serialize response, persist ProcessedCommand.
7. Handle `ErrDuplicate` from step 6 as concurrent idempotency race.

The Client and Asset lookups happen **outside** the transaction to minimize transaction duration and lock scope.

## HTTP Handler Design

### Inbound Adapter

Thin handler in `internal/adapters/inbound/httphandler/` (per hexagonal architecture — handler logic separated from bootstrap):
- Uses `events.APIGatewayV2HTTPRequest` / `events.APIGatewayV2HTTPResponse`.
- Routes `POST` to the deposit service. Returns `405` for other methods.
- Parses the event body and delegates to `DepositService.Execute`.
- Maps the returned status code and response/error to the API Gateway response.
- Does not contain business logic.
- Accepts the `DepositService` as a constructor dependency.

### Bootstrap

`cmd/http-lambda/main.go`:
1. Load `DatabaseConfig` from env.
2. Create `pgxpool.Pool` via `platform.NewPool`.
3. Instantiate repositories and `TransactionRunner`.
4. Instantiate `DepositService`.
5. Instantiate `DepositHandler`.
6. Register Lambda handler via `lambda.Start`.

## Error Mapping

| Domain / Application condition | HTTP Status |
|---|---|
| `ValidationError` (domain or application) | `422` |
| `ErrNotFound` from Client or Asset lookup | `422` (presented as validation: entity not found) |
| `ErrDuplicate` from ProcessedCommand (resolved by re-read) | `200` |
| `ErrDuplicate` from ProcessedCommand (re-read fails) | `409` |
| Any unexpected error | `500` |

`ErrNotFound` from Client/Asset is mapped to `422` (not `404`) because it is a validation failure on the deposit request input, not a resource-not-found on the deposit endpoint itself.

## Key Design Decisions

1. **Strictly positive validation at application layer, not domain layer.** `NewPosition` validates `amount >= 0` and `unit_price >= 0`. The deposit feature requires `> 0`. This stricter check lives in the application service to avoid breaking the domain contract for other potential use cases (e.g., zero-amount adjustments).

2. **Client and Asset lookups outside the transaction.** These are read-only lookups that do not need transactional guarantees with the write path. Keeping them outside reduces transaction duration and connection hold time.

3. **Transaction wraps only Position + ProcessedCommand creation.** The atomicity requirement is that a Position is never created without its corresponding idempotency record, and vice versa.

4. **Response snapshot is the position response DTO serialized as JSON.** Stored in `processed_commands.response_snapshot` (JSONB). On idempotent replay, the snapshot is deserialized directly into the response — no Position re-read needed.

5. **Concurrent idempotency race resolved by re-read.** If two concurrent deposits with the same `order_id` race, the loser gets `ErrDuplicate` from the unique index. It then re-reads the winner's `ProcessedCommand` and returns that snapshot. This avoids requiring distributed locks.

6. **Decimal values transmitted as strings.** Preserves precision across JSON serialization. Both request and response use string representation for `amount`, `unit_price`, `total_value`, `collateral_value`, `judiciary_collateral_value`.

7. **`purchased_at` set at deposit time.** `time.Now().UTC()` is called before `NewPosition` and passed in. Not caller-provided.

8. **`aws-lambda-go` dependency required.** Not currently in `go.mod`; must be added for `events.APIGatewayV2HTTPRequest`, `events.APIGatewayV2HTTPResponse`, and `lambda.Start`.

## Non-Functional Requirements

- Sustained throughput: 1,000 req/min.
- Burst: 100 req/s.
- Latency: p95 < 200 ms, p99 < 400 ms.
- Idempotency lookup backed by unique index `idx_processed_commands_type_order` (already exists).
- Minimize database round-trips: idempotency check (1), client lookup (1), asset lookup (1), transaction with position + command inserts (1 transaction, 2 statements).
- Total: 4 DB round-trips in the non-idempotent path, 1 in the idempotent path.
- Safe for upstream retries by design.

## Scope

### In Scope

- `POST` deposit endpoint via API Gateway HTTP API v2 → Lambda.
- Application service with deposit orchestration, validation, and idempotency.
- Request/response DTOs and JSON serialization.
- Lambda bootstrap wiring in `cmd/http-lambda/main.go`.
- Error mapping from domain to HTTP.
- Tests for application service and HTTP handler.

### Out of Scope

- Withdraw logic.
- Read/query endpoints.
- Collateral or judiciary value assignment beyond zero defaults.
- Asset or Client creation via this endpoint.
- Authorization / authentication.
- Pricing, accrual, tax, or settlement behavior.
- Observability instrumentation (separate concern).
- Database migrations (schema already supports the feature).

## Constraints and Assumptions

- Go 1.25.0, no ORM, no heavy HTTP frameworks.
- Aurora PostgreSQL with existing schema from `001_initial_schema.sql`.
- `github.com/aws/aws-lambda-go` will be added as a dependency.
- The existing `TransactionRunner` (UnitOfWork) is used for atomic writes.
- All repositories already implemented and tested.
- The domain model (`Position`, `ProcessedCommand`, `Client`, `Asset`) is stable and not modified by this feature.

## Implementation Layout

### New files

| Path | Purpose |
|---|---|
| `internal/application/deposit_service.go` | `DepositService` struct, `DepositRequest` / `DepositResponse` DTOs, `Execute` method |
| `internal/application/deposit_service_test.go` | Unit tests with mocked port interfaces |
| `internal/adapters/inbound/httphandler/deposit_handler.go` | `DepositHandler` struct, `Handle` method mapping API Gateway v2 events |
| `internal/adapters/inbound/httphandler/deposit_handler_test.go` | Handler mapping and routing tests |
| `cmd/http-lambda/main.go` | Bootstrap wiring (modify existing stub) |

### Package naming

- `application` — application use-case package (`internal/application/`)
- `httphandler` — HTTP inbound adapter package (`internal/adapters/inbound/httphandler/`)

### DTO ownership

- `DepositRequest` and `DepositResponse` live in `internal/application/` — they are the application service contract.
- The HTTP handler deserializes the raw event body into `DepositRequest` and serializes `DepositResponse` (or error) into the API Gateway response body.
- The response snapshot stored in `ProcessedCommand` is the JSON-serialized `DepositResponse` (via `encoding/json`).

## Open Questions

None — requirements are sufficiently specified for architecture and implementation.

## Success Criteria

1. A `POST` request with valid payload creates a new Position and returns `201`.
2. A repeated `POST` with the same `order_id` returns the stored Position with `200`.
3. Invalid requests (missing fields, non-positive amounts, unknown client/asset, unsupported product type) return `422` with a descriptive error.
4. Concurrent duplicate `order_id` requests do not create duplicate positions.
5. Position and ProcessedCommand are always persisted atomically.
6. The Lambda handler contains no business logic.
7. All changed lines have test coverage.

## Technical Debt

### Structured logging (deferred)

Per repository logging and observability skills, the service should emit structured logs at key decision boundaries (deposit created, idempotent replay, validation failure, unexpected error) using `log/slog` with JSON output. This is explicitly out of scope for the current feature per the approved design. A follow-up task should add structured logging to the application service, inbound handler, and bootstrap.

### Application service returning status codes (accepted trade-off)

The `Execute` method returns `(response, statusCode, error)`, which pushes HTTP-awareness into the application layer. Per the hexagonal architecture skill, application code should return domain/application results and let the inbound adapter determine the transport status code. The current design accepts this trade-off to keep the handler maximally thin. If additional inbound triggers (e.g., SQS) are added later, this coupling should be revisited.
