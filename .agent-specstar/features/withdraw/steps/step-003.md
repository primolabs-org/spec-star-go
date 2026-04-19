# Step 003 — Lambda bootstrap routing

## Objective

Modify `cmd/http-lambda/main.go` to instantiate `WithdrawService` and `WithdrawHandler`, and replace the single-handler `lambda.Start` with a path-based router that dispatches to both deposit and withdrawal handlers.

## Dependencies

- Step 001 must be complete (`WithdrawService` and `NewWithdrawService` must exist).
- Step 002 must be complete (`WithdrawHandler` and `NewWithdrawHandler` must exist).

## In Scope

- Instantiate `WithdrawService` with `clients`, `positions`, `processedCommands`, `unitOfWork` (no `assets` needed — the withdraw service does not use `AssetRepository`).
- Instantiate `WithdrawHandler` with the withdraw service.
- Replace `lambda.Start(handler.Handle)` with `lambda.Start(route)`.
- `route` is a package-level function (not exported) with signature `func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)`.
- `route` captures `depositHandler` and `withdrawHandler` via closure (assign both to package-level variables, or define `route` as a closure inside `main()` — either approach is acceptable as long as the routing function is clean).
- Routing logic:
  - `req.RequestContext.HTTP.Path == "/deposits"` → `depositHandler.Handle(ctx, req)`
  - `req.RequestContext.HTTP.Path == "/withdrawals"` → `withdrawHandler.Handle(ctx, req)`
  - Default → `404` response with `{"error": "not found"}` and `Content-Type: application/json`
- Rename the local variable `handler` (currently `httphandler.NewDepositHandler(service)`) to `depositHandler` to disambiguate from `withdrawHandler`.
- Rename the local variable `service` (currently `application.NewDepositService(...)`) to `depositService` to disambiguate from `withdrawService`.
- Existing deposit behavior is unchanged — same handler, same routing, same dependencies.

## Out of Scope

- Business logic (owned by `WithdrawService`).
- Handler mapping logic (owned by `WithdrawHandler`).
- Structured logging setup.
- Observability instrumentation.
- Unit tests for `main.go` (bootstrap wiring is integration-testable only).

## Required Reads

- `cmd/http-lambda/main.go` — current bootstrap code to modify.
- `internal/application/withdraw_service.go` — `NewWithdrawService` constructor signature (from step 001).
- `internal/adapters/inbound/httphandler/withdraw_handler.go` — `NewWithdrawHandler` constructor signature (from step 002).
- `github.com/aws/aws-lambda-go/events` — `APIGatewayV2HTTPRequest` and `APIGatewayV2HTTPResponse` types.

## Allowed Write Paths

- `cmd/http-lambda/main.go`

## Forbidden Paths

- `internal/application/` — complete from step 001.
- `internal/adapters/inbound/httphandler/` — complete from step 002.
- `internal/domain/` — stable, not modified.
- `internal/ports/` — stable, not modified.
- `internal/adapters/outbound/` — existing implementations, not modified.
- `internal/platform/` — existing implementation, not modified.

## Known Abstraction Opportunities

None — bootstrap is intentionally flat and explicit.

## Allowed Abstraction Scope

None.

## Required Tests

No unit tests for `main.go` — it is pure wiring with no complex conditional logic beyond the route switch. The route function is a thin dispatcher validated by integration testing and compilation verification.

Verify compilation: `go build ./cmd/http-lambda/` must succeed.

## Coverage Requirement

N/A — `main.go` is bootstrap wiring, exempt from unit test coverage. All dependencies it wires are tested in their own packages.

## Failure Model

- `LoadDatabaseConfig` failure → `log.Fatalf` (unchanged from current behavior).
- `NewPool` failure → `log.Fatalf` (unchanged from current behavior).
- Unknown route → 404 JSON response (not a Lambda invocation error).
- These are fail-fast terminal failures for bootstrap. The route 404 is an expected HTTP response.

## Allowed Fallbacks

None.

## Acceptance Criteria

1. `cmd/http-lambda/main.go` instantiates `WithdrawService` with `clients`, `positions`, `processedCommands`, `unitOfWork`.
2. `cmd/http-lambda/main.go` instantiates `WithdrawHandler` with the withdraw service.
3. `lambda.Start` receives the `route` function instead of a single handler's `Handle` method.
4. `/deposits` dispatches to `depositHandler.Handle`.
5. `/withdrawals` dispatches to `withdrawHandler.Handle`.
6. Unknown paths return `404` with `{"error": "not found"}`.
7. Bootstrap failures continue to terminate the process via `log.Fatalf`.
8. Existing deposit functionality is unchanged.
9. `go build ./cmd/http-lambda/` succeeds.
10. `go vet ./cmd/http-lambda/` produces no warnings.
11. `go build ./...` succeeds (full project compiles).

## Deferred Work

- Structured logging for bootstrap and routing (separate concern, per design.md).
- The route function is minimal and does not warrant its own test file. If routing logic grows in future features, extraction to a testable router package may become appropriate.

## Escalation Conditions

- If `NewWithdrawService` constructor signature differs from what is expected (takes different or additional dependencies), verify step 001 output before wiring.
- If `NewWithdrawHandler` constructor signature differs from what is expected, verify step 002 output before wiring.
- If the `events` import is not already present in `main.go`, it must be added for the `route` function signature and the 404 response construction.
