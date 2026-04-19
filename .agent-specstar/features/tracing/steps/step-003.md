# Step 003 — Docker Compose observability stack

## Metadata
- Feature: tracing
- Step: step-003
- Status: pending
- Depends On: none
- Last Updated: 2026-04-19

## Objective

Extend the existing Docker Compose file with the observability services (`otel-collector`, `jaeger`, `loki`, `alloy`, `grafana`) and create their configuration files so the full local stack starts with `docker compose up --build`. Rename the `lambda` service to `wallet-api` and add OTel environment variables.

## In Scope

- Modify `docker-compose.yml`:
  - Rename `lambda` service to `wallet-api`.
  - Add OTel environment variables to `wallet-api`: `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`.
  - Add `otel-collector` service (explicit image tag, e.g., `otel/opentelemetry-collector-contrib:0.102.0`) with OTLP gRPC receiver, config volume mount, health check.
  - Add `jaeger` service (explicit image tag, e.g., `jaegertracing/all-in-one:1.58`) with OTLP receiver enabled, UI port `16686`.
  - Add `loki` service (explicit image tag, e.g., `grafana/loki:3.1.0`) with local config volume mount.
  - Add `alloy` service (explicit image tag, e.g., `grafana/alloy:v1.2.0`) with config volume mount and Docker socket mount.
  - Add `grafana` service (explicit image tag, e.g., `grafana/grafana:11.1.0`) with provisioning volume mount, port `3000`.
  - Define appropriate `depends_on` ordering: `wallet-api` depends on `postgres` (healthy) and `otel-collector` (started); `alloy` depends on `loki` (started); `grafana` depends on `loki` (started).
- Create `local-env/otel-collector-config.yaml`:
  - Receivers: OTLP gRPC on `0.0.0.0:4317`.
  - Exporters: OTLP to `jaeger:4317` for traces; `debug` exporter for metrics (logging to stdout).
  - Service pipelines: `traces` (otlp → otlp/jaeger), `metrics` (otlp → debug).
- Create `local-env/loki-config.yaml`:
  - Minimal local-mode Loki config (filesystem storage, single-node, no replication).
- Create `local-env/alloy-config.alloy`:
  - Docker log discovery and scraping.
  - Forward logs to `loki:3100`.
  - Filter to include at least the `wallet-api` container.
- Create `local-env/grafana/provisioning/datasources/datasources.yaml`:
  - Preconfigure Loki data source pointing to `http://loki:3100`.
  - Preconfigure Jaeger data source pointing to `http://jaeger:16686`.
- Update `.env` or `.env.example` with new variables: `OTEL_SERVICE_NAME=spec-star-wallet`, `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317`, `OTEL_EXPORTER_OTLP_PROTOCOL=grpc`, `GRAFANA_HOST_PORT=3000`, `JAEGER_HOST_PORT=16686`.

## Out of Scope

- Any Go application code changes.
- OTel bootstrap module (`internal/platform/otel.go`).
- Trace-log correlation.
- Handler, service, or repository instrumentation.
- README update (step-007).

## Required Reads

- `.agent-specstar/features/tracing/design.md` — FR-7 through FR-12, Docker Compose signal flow.
- `docker-compose.yml` — current Compose structure.
- `Dockerfile` — understand build context.
- `local-env/` — existing local environment files.

## Allowed Write Paths

- `docker-compose.yml` (MODIFY)
- `local-env/otel-collector-config.yaml` (CREATE)
- `local-env/loki-config.yaml` (CREATE)
- `local-env/alloy-config.alloy` (CREATE)
- `local-env/grafana/provisioning/datasources/datasources.yaml` (CREATE)
- `.env` or `.env.example` (MODIFY if exists, CREATE if not)

## Forbidden Paths

- `cmd/**`
- `internal/**`
- `Dockerfile`
- `migrations/**`

## Known Abstraction Opportunities

- None. Configuration files are inherently concrete.

## Allowed Abstraction Scope

- None.

## Required Tests

This step involves infrastructure configuration only. No Go unit tests are required.

Validation is manual or via e2e script:
1. `docker compose config` validates the Compose file syntax.
2. `docker compose up --build` starts all services without errors.
3. The OTel Collector health endpoint responds (if configured).
4. Grafana is accessible on port 3000 with preconfigured data sources.
5. Jaeger UI is accessible on port 16686.

## Coverage Requirement

N/A — no Go code changes.

## Failure Model

- If the OTel Collector config is invalid, the collector container fails to start. This is visible in `docker compose logs`.
- If Alloy cannot access the Docker socket, log scraping fails silently. This is a known limitation documented in the README step.
- If Loki config is invalid, Loki fails to start. Visible in compose logs.
- If Grafana provisioning file is invalid, data sources are not preconfigured. Developer must add them manually.

## Allowed Fallbacks

- none

## Acceptance Criteria

1. `docker compose config` succeeds with no syntax errors.
2. `docker compose up --build` starts `postgres`, `wallet-api`, `otel-collector`, `jaeger`, `loki`, `alloy`, and `grafana`.
3. Grafana is accessible at `http://localhost:3000` with Loki and Jaeger data sources preconfigured.
4. Jaeger UI is accessible at `http://localhost:16686`.
5. The `wallet-api` service starts with OTel environment variables set.
6. All services use explicit image tags (not `latest`).
7. The existing `e2e-test.sh` script still works after the `lambda` → `wallet-api` rename (update the script if needed).

## Deferred Work

- none

## Escalation Conditions

- If the Alloy Docker log discovery requires privileged mode or specific Docker Desktop settings on Windows/macOS, document as a known limitation.
- If the chosen OTel Collector image tag does not support the required receiver/exporter combination, select an alternative version.
