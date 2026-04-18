# Task Routing — go-lambda-observability-otel

Use this skill for tasks such as:

- add OpenTelemetry provider bootstrap to a Go Lambda service
- instrument API Gateway HTTP adapter spans and latency metrics
- instrument SQS batch or record processing with retry-aware metrics
- wire AWS SDK v2 clients to TracerProvider and MeterProvider
- preserve correlation through HTTP or SQS flows
- prepare vendor-neutral OTLP-ready telemetry wiring

Do not use this skill alone for tasks such as:

- redesign application log schema or log levels only
- classify domain errors without telemetry changes
- broad vendor migration plans with no code changes

Combine with:

- `go-lambda-structured-logging` when logs or log correlation change
- `go-lambda-error-handling` when error mapping/status behavior changes along with traces or metrics
- `go-aws-lambda-microservice-hexagonal` when the task also shapes service architecture or adapter boundaries
