# Step 006 — Platform database configuration

## Goal

Implement pgxpool configuration and connection pool creation for Lambda execution, sourced from environment variables.

## Why

Outbound adapters (step 007) need a configured database connection pool. The pool must be tuned for Lambda's single-concurrency, freeze/thaw execution model as described in design.md. Centralizing pool configuration in `internal/platform/` follows the hexagonal skill's rule that bootstrap and config live in the platform layer.

## Depends on

Step 001 (pgx dependency declared in go.mod).

## Required Reads

- `design.md` — "Connection Management" section for pool parameter guidelines (max connections, min connections, lifetimes, idle time, health check period).
- go-aws-lambda-microservice-hexagonal skill — platform layer responsibilities.
- go-lambda-structured-logging skill — logger bootstrap (reference only; no logger implementation in this step unless trivially needed for pool error reporting).

## In Scope

### Database configuration (`internal/platform/database.go`)

- A configuration struct holding pool parameters: connection string (DSN), max connections, min connections, max connection lifetime, max connection idle time, health check period.
- A function to load configuration from environment variables, with explicit validation of required values (fail-fast on missing or invalid config).
- A function to create a `*pgxpool.Pool` from the configuration struct.
- Pool creation follows Lambda guidelines from design.md: small max connections (2–5), zero min connections, bounded lifetime and idle time.

### Environment variables

- `DATABASE_URL` (or equivalent) for the connection string — required.
- Optional overrides for pool tuning parameters with sensible Lambda-appropriate defaults from design.md.

### Design constraints

- Pool creation is a one-time cold-start operation; the pool is reused across warm invocations.
- Configuration validation is fail-fast: missing `DATABASE_URL` or invalid numeric values produce a clear error.
- No ORM, no connection abstraction layer — direct `pgxpool` usage.

## Out of Scope

- Repository implementations (step 007).
- Transaction runner / UnitOfWork implementation (step 007).
- Logger bootstrap (deferred to handler/bootstrap feature).
- Graceful shutdown coordination (deferred to Lambda handler wiring).
- TLS or IAM authentication configuration (deferred to deployment feature).
- Health check endpoints.

## Files to Create

- `internal/platform/database.go`
- `internal/platform/database_test.go`

## Forbidden Paths

- `internal/domain/`
- `internal/ports/`
- `internal/adapters/`
- `internal/application/`

## Required Tests

### `internal/platform/database_test.go`

- Valid configuration: all required env vars set, config struct populated correctly.
- Missing `DATABASE_URL`: returns a clear error.
- Invalid numeric override (e.g., `max_connections` = "abc"): returns a clear error.
- Default values: when optional overrides are absent, Lambda-appropriate defaults are applied.
- Pool configuration: verify that the `pgxpool.Config` produced from the config struct has the expected max connections, min connections, and lifetime settings.

Note: tests that create an actual pool connection require a running PostgreSQL instance and must be guarded by a build tag (e.g., `//go:build integration`). Unit tests should verify config parsing and `pgxpool.Config` construction without requiring a live database.

## Coverage Requirement

100% on all new lines (unit tests only; integration tests are additive).

## Acceptance Criteria

- `go build ./...` succeeds.
- `go test ./internal/platform/...` passes (unit tests, no database required).
- Configuration loads from environment variables with fail-fast validation.
- Pool parameters match Lambda execution model guidelines from design.md.
- No domain, port, or adapter imports in `internal/platform/database.go` (only pgx and standard library).

## Deferred Work

- TLS/IAM authentication configuration is deferred to the deployment/infrastructure feature.
- Logger injection into pool config (for pgx query logging) is deferred to the logging bootstrap feature.
- Graceful pool shutdown is deferred to Lambda handler wiring.

## Escalation Conditions

- If the connection string format or authentication mechanism is unclear, escalate for environment-specific guidance.
