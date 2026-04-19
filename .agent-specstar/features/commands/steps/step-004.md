# Step 004 — Lambda bootstrap wiring

## Objective

Wire `cmd/http-lambda/main.go` to initialize the database pool, repositories, deposit service, handler, and start the Lambda runtime.

## In Scope

- Replace the empty `main()` stub with full bootstrap:
  1. `platform.LoadDatabaseConfig()` — load DB config from environment.
  2. `platform.NewPool(ctx, cfg)` — create pgxpool connection pool.
  3. Instantiate `postgres.NewClientRepository(pool)`.
  4. Instantiate `postgres.NewAssetRepository(pool)`.
  5. Instantiate `postgres.NewPositionRepository(pool)`.
  6. Instantiate `postgres.NewProcessedCommandRepository(pool)`.
  7. Instantiate `postgres.NewTransactionRunner(pool)`.
  8. Instantiate `application.NewDepositService(...)`.
  9. Instantiate `httphandler.NewDepositHandler(service)`.
  10. Call `lambda.Start(handler.Handle)`.
- Use `context.Background()` for pool creation.
- Fail fast: if `LoadDatabaseConfig` or `NewPool` fails, terminate with a fatal log (e.g., `log.Fatalf`).
- Pool creation happens outside the handler (cold start only, reused across warm invocations).

## Out of Scope

- Business logic, validation (owned by `DepositService`).
- Handler mapping logic (owned by `DepositHandler`).
- Structured logging setup (deferred — use `log.Fatalf` for fatal bootstrap errors only).
- Graceful pool shutdown (Lambda manages process lifecycle).
- Observability instrumentation.

## Required Reads

- `cmd/http-lambda/main.go` — existing stub to replace.
- `internal/platform/database.go` — `LoadDatabaseConfig`, `NewPool` signatures.
- `internal/adapters/outbound/postgres/transaction.go` — `NewTransactionRunner` signature.
- `internal/adapters/outbound/postgres/asset_repository.go` — constructor signature.
- `internal/adapters/outbound/postgres/client_repository.go` — constructor signature.
- `internal/adapters/outbound/postgres/position_repository.go` — constructor signature.
- `internal/adapters/outbound/postgres/processed_command_repository.go` — constructor signature.
- `internal/application/deposit_service.go` — `NewDepositService` constructor signature (from step 002).
- `internal/adapters/inbound/httphandler/deposit_handler.go` — `NewDepositHandler` constructor signature (from step 003).

## Allowed Write Paths

- `cmd/http-lambda/main.go`

## Forbidden Paths

- `internal/application/` — complete from step 002.
- `internal/adapters/inbound/httphandler/` — complete from step 003.
- `internal/domain/` — stable, not modified.
- `internal/ports/` — stable, not modified.
- `internal/adapters/outbound/` — existing implementations, not modified.
- `internal/platform/` — existing implementation, not modified.

## Known Abstraction Opportunities

None — bootstrap is intentionally flat and explicit.

## Allowed Abstraction Scope

None.

## Required Tests

No unit tests for `main.go` — it is pure wiring with no conditional logic. The function calls `log.Fatalf` on bootstrap failure and `lambda.Start` on success. Both are side-effecting runtime boundaries that are validated by integration testing, not unit tests.

Verify compilation: `go build ./cmd/http-lambda/` must succeed.

## Coverage Requirement

N/A — `main.go` is bootstrap wiring, exempt from unit test coverage. All dependencies it wires are tested in their own packages.

## Failure Model

- `LoadDatabaseConfig` failure → `log.Fatalf` (process exits, Lambda reports cold start failure).
- `NewPool` failure → `log.Fatalf` (process exits, Lambda reports cold start failure).
- These are fail-fast terminal failures. No recovery, no fallback.

## Allowed Fallbacks

None.

## Acceptance Criteria

1. `cmd/http-lambda/main.go` initializes all dependencies in the correct order.
2. `main()` passes all required repositories and UnitOfWork to `NewDepositService`.
3. `main()` passes the service to `NewDepositHandler`.
4. `main()` calls `lambda.Start(handler.Handle)`.
5. Bootstrap failures terminate the process via `log.Fatalf`.
6. `go build ./cmd/http-lambda/` succeeds.
7. `go vet ./cmd/http-lambda/` produces no warnings.

## Deferred Work

- Structured logging for bootstrap (covered in design.md Technical Debt section).
- Graceful pool shutdown is not needed — Lambda manages process lifecycle.

## Escalation Conditions

- If any repository constructor signature has changed since the last commit, verify before wiring.
- If `lambda.Start` requires a different function signature than `func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)`, verify against `aws-lambda-go` documentation before proceeding.
