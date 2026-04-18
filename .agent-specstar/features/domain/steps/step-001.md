# Step 001 — Go module initialization and project scaffolding

## Goal

Initialize the Go module with declared dependencies and create the hexagonal directory structure.

## Why

All subsequent steps need a valid Go module and the correct package layout. Declaring dependencies upfront ensures later steps can import them without additional setup.

## Depends on

None.

## Required Reads

- `design.md` — "Constraints and Allowed Libraries" and "Connection Management" sections.
- `clarifications.md` — open decisions on `shopspring/decimal` and `google/uuid` approval status.

## In Scope

- `go.mod` with module path matching the repository URL, Go 1.24+ directive, and dependency declarations.
- Dependencies to declare: `github.com/jackc/pgx/v5` (includes pgxpool), `github.com/shopspring/decimal`, `github.com/google/uuid`.
- Minimal `cmd/http-lambda/main.go` stub (package main, empty main function) so `go build ./...` succeeds.
- Other directories (`internal/domain/`, `internal/application/`, `internal/ports/`, `internal/adapters/inbound/`, `internal/adapters/outbound/`, `internal/platform/`, `migrations/`) are created by subsequent steps as files are added.

## Out of Scope

- Domain types, entities, ports, adapters, or platform code.
- Business logic of any kind.
- Test files.
- Migration SQL files.
- Linter configuration, CI/CD, Makefile, or build scripts.
- Logging, observability, or error handling.

## Files to Create

- `go.mod`
- `cmd/http-lambda/main.go`

## Forbidden Paths

- `internal/` — no source files in internal packages yet.
- `migrations/` — no migration files yet.

## Required Tests

None. Acceptance is verified by successful compilation.

## Coverage Requirement

N/A — no testable logic.

## Acceptance Criteria

- `go build ./...` succeeds with no errors.
- `go vet ./...` succeeds with no errors.
- `go.mod` declares `go 1.24` (or higher) and lists `pgx/v5`, `shopspring/decimal`, and `google/uuid` as dependencies.

## Escalation Conditions

- If `shopspring/decimal` or `google/uuid` approval is not confirmed, escalate before proceeding. All subsequent steps assume these libraries are approved per clarifications.md.
