# Step 004 - Add Path Routing in main.go

## Metadata
- Feature: withdraw
- Step: step-004
- Status: pending
- Depends On: step-003
- Last Updated: 2026-04-19

## Objective

Wire `WithdrawService` and `WithdrawHandler` in the Lambda bootstrap, and add path-based routing that dispatches `/deposits` to the deposit handler and `/withdrawals` to the withdraw handler.

## In Scope

- Modify `cmd/http-lambda/main.go` to:
  - Construct `WithdrawService` using existing repository and unit-of-work instances (same pool and adapter instances used by deposit).
  - Construct `WithdrawHandler` with the `WithdrawService`.
  - Replace `lambda.Start(handler.Handle)` with a router function that dispatches by `req.RequestContext.HTTP.Path`:
    - `/deposits` → `depositHandler.Handle(ctx, req)`
    - `/withdrawals` → `withdrawHandler.Handle(ctx, req)`
    - Default → `404 Not Found` JSON response.
  - The router is a `func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)` passed to `lambda.Start`.

### Router Shape

```go
lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
    switch req.RequestContext.HTTP.Path {
    case "/deposits":
        return depositHandler.Handle(ctx, req)
    case "/withdrawals":
        return withdrawHandler.Handle(ctx, req)
    default:
        return events.APIGatewayV2HTTPResponse{
            StatusCode: 404,
            Headers:    map[string]string{"Content-Type": "application/json"},
            Body:       `{"error":"not found"}`,
        }, nil
    }
})
```

### Dependency Wiring

`WithdrawService` does not need `AssetRepository` (unlike `DepositService`). It needs:
- `clients` (existing `ClientRepository` instance)
- `positions` (existing `PositionRepository` instance)
- `processedCommands` (existing `ProcessedCommandRepository` instance)
- `unitOfWork` (existing `TransactionRunner` instance)

## Out of Scope

- Changes to service or handler implementations.
- Changes to domain or ports.
- Tests for the routing logic (the router is a thin `switch` statement in `main.go`; each handler is individually tested).

## Required Reads

- `cmd/http-lambda/main.go` — current bootstrap and handler wiring.
- `internal/application/withdraw_service.go` — `NewWithdrawService` constructor signature.
- `internal/adapters/inbound/httphandler/withdraw_handler.go` — `NewWithdrawHandler` constructor signature.

## Allowed Write Paths

- `cmd/http-lambda/main.go`

## Forbidden Paths

- `internal/`
- Any other file under `cmd/` besides `cmd/http-lambda/main.go`.

## Known Abstraction Opportunities

None. A simple `switch` statement is appropriate for two routes.

## Allowed Abstraction Scope

None. Do not introduce a route table, registry, or router abstraction.

## Required Tests

None. The router is a three-way `switch` in a Lambda bootstrap function. Each handler is individually tested. Integration testing of Lambda dispatch is out of scope.

## Coverage Requirement

100% on changed lines where unit-testable. The `main()` function is exempt from unit test coverage per standard Go convention.

## Failure Model

- Unknown path → 404 JSON response.
- Wiring error → compile-time failure.
- `WithdrawService` constructor missing a dependency → compile-time failure.

## Allowed Fallbacks

None.

## Acceptance Criteria

1. `cmd/http-lambda/main.go` constructs `WithdrawService` and `WithdrawHandler` with the correct dependencies.
2. Lambda handler dispatches `/deposits` to `depositHandler.Handle` and `/withdrawals` to `withdrawHandler.Handle`.
3. Unrecognized paths produce a `404 Not Found` JSON response with `{"error":"not found"}`.
4. Existing deposit functionality is unchanged.
5. The project compiles and all existing tests pass.
6. No new dependencies introduced.
7. No framework or router abstraction introduced — plain `switch` statement.

## Deferred Work

None.

## Escalation Conditions

- If `events.APIGatewayV2HTTPRequest.RequestContext.HTTP.Path` does not contain the expected path value at runtime (e.g., includes stage prefix), escalate — the routing strategy depends on this field's format.
