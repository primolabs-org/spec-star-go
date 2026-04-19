# Step 001 — Platform: OTel bootstrap and main.go integration

## Metadata
- Feature: tracing
- Step: step-001
- Status: pending
- Depends On: none
- Last Updated: 2026-04-19

## Objective

Create the OpenTelemetry bootstrap module in `internal/platform/` and integrate it into the Lambda entry point so that a `TracerProvider` and `MeterProvider` are initialized at startup and shut down cleanly before exit.

## In Scope

- New `internal/platform/otel.go` with an exported `InitTelemetry(ctx context.Context) (shutdown func(context.Context) error, err error)` function.
- Build a `resource.Resource` with `service.name` (from `OTEL_SERVICE_NAME`), `service.version`, and `deployment.environment` (from `OTEL_RESOURCE_ATTRIBUTES`).
- Create an OTLP gRPC trace exporter and construct a `TracerProvider` using it.
- Create an OTLP gRPC metric exporter and construct a `MeterProvider` using it.
- Register global providers via `otel.SetTracerProvider` and `otel.SetMeterProvider`.
- Set global text map propagator (`propagation.TraceContext{}`).
- Return a composite shutdown function that flushes and closes both providers.
- Modify `cmd/http-lambda/main.go` to call `InitTelemetry` at startup, log and `os.Exit(1)` on failure, and `defer shutdown(ctx)` on success.
- Add all required OTel Go module dependencies to `go.mod` / `go.sum` via `go get`.
- Full unit test coverage for the bootstrap module.

## Out of Scope

- Trace-log correlation (step-002).
- Span creation in handlers, services, or repositories (steps 004–006).
- Docker Compose changes (step-003).
- Any metric or span recording at this step.

## Required Reads

- `.agent-specstar/features/tracing/design.md` — FR-1, Technical Approach (OTel bootstrap), Required OTel Go packages table.
- `internal/platform/database.go` — existing platform package structure and patterns.
- `internal/platform/logger.go` — understand current bootstrap flow.
- `cmd/http-lambda/main.go` — current startup sequence.
- `.github/skills/go-lambda-observability-otel/SKILL.md` — bootstrap/platform layer rules.
- `go.mod` — current dependencies.

## Allowed Write Paths

- `internal/platform/otel.go` (CREATE)
- `internal/platform/otel_test.go` (CREATE)
- `cmd/http-lambda/main.go` (MODIFY — add OTel init call and deferred shutdown)
- `go.mod` (MODIFY — add OTel dependencies)
- `go.sum` (MODIFY — updated by `go get`)

## Forbidden Paths

- `internal/platform/logger.go`
- `internal/platform/logger_test.go`
- `internal/adapters/**`
- `internal/application/**`
- `internal/domain/**`
- `internal/ports/**`
- `docker-compose.yml`

## Known Abstraction Opportunities

- None. `InitTelemetry` is the minimal stable abstraction for OTel bootstrap.

## Allowed Abstraction Scope

- None beyond what is specified.

## Required Tests

All in `internal/platform/otel_test.go`:

1. `InitTelemetry` with valid environment configuration returns a non-nil shutdown function and no error.
2. The returned shutdown function does not error when called (clean shutdown path).
3. The global `TracerProvider` is set after `InitTelemetry` (verify `otel.GetTracerProvider()` is not the noop provider).
4. The global `MeterProvider` is set after `InitTelemetry` (verify `otel.GetMeterProvider()` is not the noop provider).
5. The resource created by `InitTelemetry` includes the configured `service.name` attribute.
6. Tests must use environment variable overrides (e.g., `t.Setenv`) to configure OTLP endpoints pointing to a non-existent address or use in-memory/noop exporters. Tests must NOT require a running OTel Collector.

## Coverage Requirement

100% on all lines in `internal/platform/otel.go`.

## Failure Model

- If `InitTelemetry` fails (exporter creation, resource build, provider construction), it returns an error. The caller (`main.go`) logs the error and exits with `os.Exit(1)`.
- OTel bootstrap failure is a hard startup failure (fail-fast). The application does not start in a degraded mode without telemetry.

## Allowed Fallbacks

- none

## Acceptance Criteria

1. `internal/platform/otel.go` compiles with no errors.
2. `go test ./internal/platform/...` passes with 100% coverage on `otel.go`.
3. `cmd/http-lambda/main.go` calls `InitTelemetry` and defers shutdown.
4. `go build ./cmd/http-lambda/` succeeds.
5. All OTel Go dependencies are present in `go.mod`.
6. No exporter endpoint or service identity is hard-coded; all values come from environment variables.

## Deferred Work

- none

## Escalation Conditions

- OTel Go SDK v1.x vs v0.x API instability for metric exporters — verify the chosen package versions are compatible.
- If `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` has breaking API differences from the trace exporter equivalent, document the discrepancy.
