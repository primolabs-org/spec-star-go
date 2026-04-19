# Observability: Tracing, Metrics, and Local Observability Stack

## Metadata
- Feature: tracing
- Status: draft
- Owner: SpecStar
- Last Updated: 2026-04-19
- Source Request: Implement OpenTelemetry-based tracing and metrics for the fixed-income wallet Lambda, and extend the Docker Compose local environment with an observable signal pipeline (OTel Collector, Jaeger, Loki, Alloy, Grafana).

## Problem Statement

The wallet microservice has structured logging in place but lacks distributed tracing and metrics. A developer debugging latency, failure rates, or request flow has no spans, no durations, no outcome counters, and no way to follow a request across inbound adapter → application → repository layers. The local development environment exposes only raw container stdout; there is no queryable log backend, no trace UI, and no correlation between logs and traces.

## Goal

After this feature is complete:

1. Every deposit and withdrawal request produces a distributed trace visible in Jaeger with spans covering the HTTP adapter, application use-case, and repository/database operations.
2. Command-level metrics (counters, durations, outcomes) are emitted via OpenTelemetry.
3. Structured application logs are queryable in Grafana via Loki, collected from Docker container stdout by Alloy.
4. Logs and traces share correlation identifiers (`trace_id`, `span_id`) so a developer can jump between signals.
5. The entire stack starts with `docker compose up --build` on a clean machine with only Docker installed.

## Functional Requirements

### FR-1: OTel bootstrap module

A new `internal/platform` module must initialize OpenTelemetry tracing and metrics during application startup.

- Create a `TracerProvider` configured with an OTLP gRPC exporter.
- Create a `MeterProvider` configured with an OTLP gRPC exporter.
- Set a `Resource` with attributes: `service.name`, `service.version`, `deployment.environment` (sourced from environment variables).
- Register the global tracer and meter providers.
- Return a shutdown function that flushes and closes providers; the caller in `main.go` must invoke it before exit.
- All exporter endpoints and resource attributes must come from standard OTel environment variables (`OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`).
- If OTel bootstrap fails, the application must fail to start (fail-fast). Degraded startup without telemetry is not acceptable.

### FR-2: Inbound adapter span creation

Each HTTP handler (`DepositHandler.Handle`, `WithdrawHandler.Handle`) must create a root span representing the Lambda command handling operation.

- Span name: `POST /deposits` or `POST /withdrawals` (stable, low-cardinality).
- The span must wrap the full handler execution including parsing, service call, and response mapping.
- On success, set span status OK.
- On failure (validation, business error, infrastructure error), set span status Error with a description; record the error on the span.
- Attach attributes: `http.method`, `http.route`, `wallet.command` (`deposit` or `withdraw`), `wallet.outcome` (`success`, `failed`, `replayed`).

### FR-3: Application service span creation

Each service (`DepositService.Execute`, `WithdrawService.Execute`) must create a child span representing use-case execution.

- Span name: `deposit.execute` or `withdraw.execute`.
- Attach `wallet.order_id` as an attribute.
- Attach `wallet.outcome` (`success`, `failed`, `replayed`) when the outcome is known.
- If the request is an idempotent replay, the span must still exist but reflect the replay outcome.

### FR-4: Repository/database span creation

Important outbound database operations must be covered by child spans:

- `ProcessedCommandRepository.FindByTypeAndOrderID`
- `ClientRepository.FindByID`
- `AssetRepository.FindByID`
- `PositionRepository.FindByClientAndInstrument`
- `PositionRepository.Create`
- `PositionRepository.Update`
- `ProcessedCommandRepository.Create`
- `UnitOfWork.Do` (wrapping the transaction boundary)

Span naming convention: `db.<entity>.<operation>` (e.g., `db.position.create`, `db.processed_command.find_by_type_and_order_id`, `db.transaction`).

Attributes: `db.system` = `postgresql`, `db.operation.name` (e.g., `SELECT`, `INSERT`, `UPDATE`).

Do not attach query text, parameter values, or sensitive data as span attributes.

### FR-5: Command metrics

Emit the following metrics via a shared `Meter`:

| Metric name | Type | Attributes | Description |
|---|---|---|---|
| `wallet.command.count` | Counter | `command` (`deposit`/`withdraw`), `outcome` (`success`/`failed`/`replayed`) | Total commands processed |
| `wallet.command.duration` | Histogram | `command` (`deposit`/`withdraw`), `outcome` (`success`/`failed`/`replayed`) | End-to-end command duration in milliseconds |
| `wallet.db.duration` | Histogram | `operation` (span name) | Database operation duration in milliseconds |

Metric names must be stable. Attributes must be low-cardinality.

### FR-6: Trace-log correlation

Enhance the existing `LoggerFactory.FromContext` to inject `trace_id` and `span_id` into every log record when an active span exists in the context.

- The primary logging path remains JSON to stdout via `log/slog`. This must not change.
- If no active span exists, logs are emitted without trace fields (no failure, no placeholder values).
- Do not route application logs through the OTel Logs SDK exporter. Logs flow through Docker → Alloy → Loki only.

### FR-7: OpenTelemetry Collector

Add an OTel Collector configuration file to the repository (e.g., `local-env/otel-collector-config.yaml`).

- Receiver: OTLP (gRPC on port 4317).
- Exporter: OTLP to Jaeger.
- Pipeline: `traces` receiver → exporter.
- The configuration must be minimal and explicit for local development.

### FR-8: Jaeger backend

Add a Jaeger service to Docker Compose.

- Receive traces from the OTel Collector via OTLP.
- Expose the Jaeger UI on a configurable host port (default: `16686`).
- Use an explicit image tag (not `latest`).

### FR-9: Loki backend

Add a Loki service to Docker Compose.

- Accept log pushes from Alloy.
- Use a minimal local-mode configuration suitable for development (e.g., `local-env/loki-config.yaml`).
- Use an explicit image tag.

### FR-10: Alloy log collector

Add a Grafana Alloy service to Docker Compose.

- Scrape Docker container logs (specifically from `wallet-api` at minimum).
- Forward logs to Loki.
- Configuration file in the repository (e.g., `local-env/alloy-config.alloy`).
- Mount the Docker socket so Alloy can discover containers.

### FR-11: Grafana

Add a Grafana service to Docker Compose.

- Preconfigure Loki as a data source (provisioning file, not manual setup).
- Optionally preconfigure Jaeger as a data source.
- Expose the Grafana UI on a configurable host port (default: `3000`).
- Use an explicit image tag.
- A developer must be able to open Grafana Explore and find `wallet-api` logs without manual configuration.

### FR-12: Docker Compose integration

Extend the existing `docker-compose.yml` (do not create a separate file).

- Rename the `lambda` service to `wallet-api` for clarity in log labels and discovery.
- Add environment variables to `wallet-api`: `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`.
- Add services: `otel-collector`, `jaeger`, `loki`, `alloy`, `grafana`.
- Define dependency ordering so `wallet-api` starts after `postgres` and `otel-collector` are healthy/started.
- The stack must start cleanly with `docker compose up --build`.

### FR-13: README update

Update the README to document:

- Updated local stack services and their ports.
- Grafana URL and how to browse logs.
- Jaeger URL and how to inspect traces.
- How to invoke deposit/withdrawal locally (existing content, verify still accurate).
- Which environment variables control OTel export.
- Known limitations of the local observability setup.

## Non-Functional Requirements

- **NFR-1**: Keep instrumentation vendor-neutral. Application code must depend only on OpenTelemetry Go APIs, not on Jaeger, Loki, or Grafana SDKs.
- **NFR-2**: Do not hard-code local-only behavior into business logic. Exporter endpoints and resource identity must come from environment variables.
- **NFR-3**: Application logs must remain visible through `docker compose logs wallet-api` even when the Collector, Jaeger, or Loki is down. The `slog` → stdout path must never be gated on OTel pipeline health.
- **NFR-4**: Telemetry initialization must be outside the hot request path. Providers are created once during bootstrap and reused across warm invocations.
- **NFR-5**: No high-cardinality or sensitive attributes in spans or metrics. Order IDs are acceptable (low-cardinality per deployment); raw request bodies, credentials, and connection strings are forbidden.
- **NFR-6**: Dependency additions must be minimal and justified. Only OpenTelemetry SDK/API packages, OTLP exporters, and resource/propagation packages are allowed by default. A pgx OTel instrumentation package or slog bridge may be used if justified.
- **NFR-7**: Test coverage on changed lines must be 100%. Overall codebase coverage must remain above 90%.
- **NFR-8**: The Compose stack must work on a clean machine with only Docker and Docker Compose installed.

## Scope

### Work stream 1: Application-side OTel instrumentation

- New `internal/platform/otel.go` — OTel bootstrap (TracerProvider, MeterProvider, Resource, shutdown).
- Modify `cmd/http-lambda/main.go` — Call OTel bootstrap at startup, defer shutdown.
- Modify `internal/adapters/inbound/httphandler/deposit_handler.go` — Root span, metrics recording.
- Modify `internal/adapters/inbound/httphandler/withdraw_handler.go` — Root span, metrics recording.
- Modify `internal/application/deposit_service.go` — Child span for use-case execution.
- Modify `internal/application/withdraw_service.go` — Child span for use-case execution.
- Modify `internal/adapters/outbound/postgres/` — Child spans for repository operations.
- Modify `internal/adapters/outbound/postgres/transaction.go` — Span around `UnitOfWork.Do`.
- Modify `internal/platform/logger.go` — Inject `trace_id`/`span_id` from active span into `slog` records.

### Work stream 2: Local Docker Compose observability stack

- New `local-env/otel-collector-config.yaml`.
- New `local-env/loki-config.yaml`.
- New `local-env/alloy-config.alloy`.
- New `local-env/grafana/provisioning/datasources/datasources.yaml`.
- Modify `docker-compose.yml` — Add `otel-collector`, `jaeger`, `loki`, `alloy`, `grafana` services; update `wallet-api` (formerly `lambda`) with OTel env vars.
- Modify `.env.example` — Add OTel and new port variables.
- Modify `README.md` — Observability documentation.

## Out of Scope

- Production-grade observability backend deployment.
- Long-term trace or log storage and retention policies.
- Alerting rules or SLO dashboards.
- Metrics backend beyond what the OTel Collector pipeline provides locally (no Prometheus/Mimir for this feature).
- Automatic instrumentation of every dependency (only explicit spans at important boundaries).
- Redesigning the application architecture.
- LocalStack or SAM; the existing Lambda RIE model is preserved.
- OTel Logs SDK export path; application logs remain stdout-only.

## Constraints and Assumptions

- **C-1**: Go toolchain version remains `1.26.2`.
- **C-2**: The existing Lambda/RIE-based local testing model must be preserved. The Dockerfile multi-stage build targeting `public.ecr.aws/lambda/provided:al2023` does not change.
- **C-3**: The lightweight SQL-first persistence approach (pgx, no ORM) must not be replaced.
- **C-4**: Repository cleanup, fail-fast, testing, error-handling, logging, and observability instructions apply to all changes.
- **C-5**: The existing `LoggerFactory` pattern, JSON structured logging, context-based logger propagation, and cold start tracking established by the `logging` feature are preserved and extended, not replaced.
- **C-6**: OTel bootstrap failure is treated as a startup failure (fail-fast), not a degraded mode.
- **A-1**: Environment variables (`OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, etc.) will be set by the Compose file for local dev and by infrastructure configuration for deployed environments.
- **A-2**: The OTel Collector runs as a sidecar-like container in Compose; in production, the export target may differ but the application code remains unchanged.
- **A-3**: Alloy can discover Docker containers and their logs via the Docker socket mount.

## Existing Context

### What already exists (from the `logging` feature)

- `internal/platform/logger.go`: `LoggerFactory` with `FromContext()` (enriches with `trigger`, `operation`, `request_id`, `cold_start`), `WithLogger()`, `LoggerFromContext()`. JSON handler on `os.Stdout`. `slog.SetDefault` called at bootstrap.
- All handlers, services, and repositories already use structured `slog` logging via `platform.LoggerFromContext(ctx)`.
- `cmd/http-lambda/main.go`: Creates `LoggerFactory`, loads DB config, creates pool, wires everything, starts Lambda.
- `docker-compose.yml`: Two services (`postgres`, `lambda`). Uses `.env` for configuration. Health check on postgres.
- `Dockerfile`: Multi-stage build. `golang:1.26.2-alpine` → `public.ecr.aws/lambda/provided:al2023`.
- `go.mod`: No OTel dependencies. Current deps: `aws-lambda-go`, `google/uuid`, `jackc/pgx/v5`, `shopspring/decimal`.

### What needs to change

- `go.mod` gains OTel dependencies (SDK, API, OTLP exporters, resource, propagation packages).
- `internal/platform/` gains an OTel bootstrap module.
- `internal/platform/logger.go` gains trace-log correlation (inject `trace_id`/`span_id`).
- `cmd/http-lambda/main.go` gains OTel initialization call and deferred shutdown.
- Both HTTP handlers gain root span creation and metric recording.
- Both application services gain child span creation.
- Postgres repository and transaction code gains child spans for database operations.
- `docker-compose.yml` gains five new services and updated `wallet-api` configuration.
- Several new config files appear in `local-env/`.
- `.env.example` gains new variables.
- `README.md` gains observability documentation.

## Technical Approach

### OTel bootstrap (`internal/platform/otel.go`)

- Exported function (e.g., `InitTelemetry(ctx) (shutdown func(context.Context) error, err error)`) that:
  - Builds a `resource.Resource` from env vars.
  - Creates OTLP gRPC trace exporter and `TracerProvider`.
  - Creates OTLP gRPC metric exporter and `MeterProvider`.
  - Registers global providers via `otel.SetTracerProvider` / `otel.SetMeterProvider`.
  - Returns a composite shutdown function.
- The function reads configuration from standard OTel env vars; no custom config struct needed.

### Trace-log correlation (`internal/platform/logger.go`)

- Enhance `FromContext` (or introduce a custom `slog.Handler` wrapper) so that when `trace.SpanFromContext(ctx)` returns a recording span, `trace_id` and `span_id` attributes are injected into every log record.
- If no span exists, no trace fields are added (no placeholders, no failure).

### Span instrumentation pattern

- Handlers use `otel.Tracer("httphandler")` to start a span, defer `span.End()`.
- Services use `otel.Tracer("application")` to start a child span.
- Repositories use `otel.Tracer("postgres")` to start a child span.
- Context propagation is already in place (`context.Context` is threaded everywhere).

### Required OTel Go packages

| Package | Purpose |
|---|---|
| `go.opentelemetry.io/otel` | API entry point |
| `go.opentelemetry.io/otel/sdk/trace` | TracerProvider |
| `go.opentelemetry.io/otel/sdk/metric` | MeterProvider |
| `go.opentelemetry.io/otel/sdk/resource` | Resource with service identity |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | OTLP trace exporter |
| `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` | OTLP metric exporter |
| `go.opentelemetry.io/otel/semconv/v1.26.0` (or latest stable) | Semantic conventions |
| `go.opentelemetry.io/otel/trace` | Trace API |
| `go.opentelemetry.io/otel/metric` | Metric API |
| `go.opentelemetry.io/otel/codes` | Span status codes |

### Docker Compose signal flow

```
wallet-api stdout/stderr ──► Docker log driver ──► Alloy ──► Loki ──► Grafana (Explore)
wallet-api OTLP (gRPC:4317) ──► OTel Collector ──► Jaeger (OTLP) ──► Jaeger UI (:16686)
```

## Affected Components

| Component | Change type |
|---|---|
| `cmd/http-lambda/main.go` | Modified — add OTel init and shutdown |
| `internal/platform/otel.go` | New — OTel bootstrap |
| `internal/platform/logger.go` | Modified — trace-log correlation |
| `internal/adapters/inbound/httphandler/deposit_handler.go` | Modified — root span, metrics |
| `internal/adapters/inbound/httphandler/withdraw_handler.go` | Modified — root span, metrics |
| `internal/application/deposit_service.go` | Modified — child span |
| `internal/application/withdraw_service.go` | Modified — child span |
| `internal/adapters/outbound/postgres/*.go` | Modified — db spans |
| `docker-compose.yml` | Modified — new services, env vars |
| `.env.example` | Modified — new variables |
| `README.md` | Modified — observability docs |
| `local-env/otel-collector-config.yaml` | New |
| `local-env/loki-config.yaml` | New |
| `local-env/alloy-config.alloy` | New |
| `local-env/grafana/provisioning/datasources/datasources.yaml` | New |

## Contracts and Data Shape Impact

No changes to request/response contracts. Deposit and withdrawal request/response DTOs remain identical. No new HTTP endpoints.

## State / Persistence Impact

No database schema changes. No new tables. No migration files.

## Failure Model

| Failure scenario | Expected behavior |
|---|---|
| OTel bootstrap fails (e.g., invalid env config) | Application fails to start (fail-fast, `os.Exit(1)` with error log) |
| OTel Collector is down | Spans and metrics fail to export; application continues processing requests; logs remain visible via stdout |
| Jaeger is down | Collector may buffer or drop traces; application is unaffected |
| Loki is down | Alloy may buffer or drop; logs remain visible via `docker compose logs` |
| Alloy is down | Logs remain visible via `docker compose logs`; Grafana shows no results |
| Grafana is down | Logs and traces still exist; developer uses `docker compose logs` and Jaeger UI directly |

Key principle: the `slog` → stdout path is never gated on OTel or backend health.

## Testing and Validation Strategy

### Unit tests

- `internal/platform/otel_test.go` — Verify `InitTelemetry` constructs providers and returns a working shutdown function. Use in-memory or noop exporters. Verify resource attributes.
- `internal/platform/logger_test.go` — Verify trace-log correlation: when a span exists in context, `trace_id` and `span_id` appear in log output; when no span exists, they do not.
- `internal/adapters/inbound/httphandler/*_test.go` — Verify spans are created with expected names and attributes. Use a test `TracerProvider` with an in-memory span exporter. Verify metrics are recorded.
- `internal/application/*_test.go` — Verify child spans are created for service execution.
- `internal/adapters/outbound/postgres/*_test.go` — Verify child spans are created for repository operations.

### Integration validation (manual / e2e script)

1. `docker compose up --build` starts all services successfully.
2. A deposit request produces a trace in Jaeger with root span `POST /deposits` and child spans for service execution and database operations.
3. A withdrawal request produces a trace in Jaeger with root span `POST /withdrawals` and child spans.
4. Structured logs for both requests appear in Grafana Explore (Loki data source) with `trace_id` and `span_id` fields.
5. The application still returns correct responses against PostgreSQL.
6. `docker compose logs wallet-api` shows structured JSON logs with trace correlation fields.

### Coverage requirement

- 100% coverage on all changed lines.
- Above 90% on overall codebase.

## Execution Notes

- Work stream 2 (Docker Compose stack) can be developed in parallel with work stream 1 (application instrumentation) since they are largely independent. The integration point is the OTLP endpoint environment variable.
- The `lambda` service in Docker Compose should be renamed to `wallet-api` to improve log label clarity across the observability stack. This is a single rename in `docker-compose.yml` and README.
- The trace-log correlation enhancement to `LoggerFactory` should be designed so that `logger.go` gains a dependency on `go.opentelemetry.io/otel/trace` (API only, not SDK). This is acceptable because the platform layer is the designated location for cross-cutting concerns.
- Repository span creation can potentially be centralized via a helper in the `postgres` package to avoid repetitive span boilerplate, but each repository call must still own its own span name.
- The `slog.Handler` wrapper approach for trace-log correlation is preferred over modifying every `FromContext` call site, because it injects trace context transparently at the handler level.

## Open Questions

1. **Metric export backend**: The feature intent specifies metrics but does not include a Prometheus or Mimir backend in the Compose stack. Should the OTel Collector export metrics anywhere for local validation, or is it sufficient that the application emits them and they can be verified via collector debug logging? **Recommendation**: Add collector debug/logging exporter for metrics in local config; defer a full metrics backend to a future feature.

2. **Alloy Docker socket access**: On some host OSes (particularly rootless Docker or Docker Desktop edge cases), mounting `/var/run/docker.sock` into a container may require specific permissions. Should the design document a fallback? **Recommendation**: Document as a known limitation in README.

3. **Grafana Jaeger data source**: Should Grafana be preconfigured with Jaeger as a data source (enabling log-to-trace navigation in the UI), or is Jaeger UI on its own port sufficient? **Recommendation**: Preconfigure both Loki and Jaeger as Grafana data sources for a unified experience.

4. **pgx OTel instrumentation package**: Should `github.com/jackc/pgx/v5` automatic query tracing be used (via `pgx`'s built-in tracer hook), or should spans be created manually in each repository method? **Recommendation**: Use manual spans for this feature to keep the dependency surface minimal and span naming explicit. Evaluate pgx tracing hook in a future iteration.

5. **Service rename scope**: Renaming `lambda` → `wallet-api` in Docker Compose will change `docker compose logs lambda` instructions in the existing README and potentially affect developer muscle memory. Is this acceptable? **Recommendation**: Yes, proceed with rename; update all references.

## Success Criteria

1. `docker compose up --build` starts `postgres`, `wallet-api`, `otel-collector`, `jaeger`, `loki`, `alloy`, and `grafana` without errors.
2. A deposit request to the Lambda RIE endpoint produces a visible trace in Jaeger UI (`http://localhost:16686`) with root and child spans.
3. A withdrawal request produces a visible trace in Jaeger UI with root and child spans.
4. Structured JSON logs from `wallet-api` appear in Grafana Explore (`http://localhost:3000`) via the Loki data source.
5. Log records include `trace_id` and `span_id` fields that match spans visible in Jaeger.
6. `wallet.command.count` and `wallet.command.duration` metrics are emitted (verifiable via collector logs or debug exporter).
7. The application returns correct deposit and withdrawal responses against PostgreSQL (existing behavior preserved).
8. All unit tests pass with 100% coverage on changed lines.
9. README documents Grafana URL, Jaeger URL, invocation examples, OTel environment variables, and known limitations.
