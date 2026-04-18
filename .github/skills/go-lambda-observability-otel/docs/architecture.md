# Architecture Notes — go-lambda-observability-otel

## Intent

This skill keeps OpenTelemetry instrumentation as a platform concern that supports a Go Lambda hexagonal microservice without turning backend/exporter details into domain code.

## Boundary ownership

- `internal/platform/observability` owns tracer and meter provider bootstrap.
- `internal/adapters/inbound/httpapi` owns inbound HTTP request span boundaries and request correlation.
- `internal/adapters/inbound/sqs` owns inbound batch and/or per-record span boundaries and retry-aware metrics.
- `internal/adapters/outbound/...` owns dependency spans and dependency-oriented metrics.
- `internal/domain/...` remains free of backend/exporter concerns.
- `internal/application/...` may add meaningful use-case spans or annotate current spans when that boundary matters operationally.

## Signal split

- Structured application logs remain owned by the logging skill.
- Traces and metrics are owned by this skill.
- Log/trace correlation may be added by bootstrap or adapter enrichment where needed.

## Recommended package placement

- `internal/platform/observability/provider.go`
- `internal/platform/observability/resource.go`
- `internal/platform/observability/shutdown.go`
- `internal/adapters/inbound/httpapi/handler.go`
- `internal/adapters/inbound/sqs/handler.go`
- `internal/adapters/outbound/<dependency>/...`

## Typical flow

1. Bootstrap config constructs resource metadata and providers.
2. Lambda entrypoint reuses bootstrap across warm invocations.
3. Inbound adapter starts or continues span context.
4. Application use case runs under that context.
5. Outbound adapters create dependency spans and emit dependency metrics where meaningful.
6. Logger enrichment may attach trace context if repository conventions require it.
