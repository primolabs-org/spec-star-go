# Fix Step 004-fix-001 - Rename deposit service variable for consistency

## Metadata
- Feature: withdraw
- Fixes: step-004
- Status: pending
- Last Updated: 2026-04-19

## Gap

In `cmd/http-lambda/main.go`, the deposit handler variable was renamed from `handler` to `depositHandler` for clarity alongside the new `withdrawHandler`. However, the deposit service variable was left as `service` while the withdraw service is named `withdrawService`. This creates a naming inconsistency in a touched area.

The cleanup instructions require: "When a refactor changes behavior or structure, update surrounding names, comments, and references so they stay accurate."

## Required Change

In `cmd/http-lambda/main.go`, rename the variable `service` to `depositService` on the line that constructs `NewDepositService`, and update its single usage when passed to `NewDepositHandler`.

Before:
```go
service := application.NewDepositService(clients, assets, positions, processedCommands, unitOfWork)
depositHandler := httphandler.NewDepositHandler(service)
```

After:
```go
depositService := application.NewDepositService(clients, assets, positions, processedCommands, unitOfWork)
depositHandler := httphandler.NewDepositHandler(depositService)
```

## Allowed Write Paths

- `cmd/http-lambda/main.go`

## Scope

Rename only. No logic changes, no new dependencies, no new abstractions.
