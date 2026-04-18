---
name: "go-lambda-observability-otel"
description: "A skill for implementing OpenTelemetry-based observability in Go microservices on AWS Lambda."
user-invocable: false
---

# SKILL: go-lambda-observability-otel

## Purpose

Implement, modify, or review OpenTelemetry-based observability in Go AWS Lambda services so traces, metrics, and correlation context are emitted consistently across hexagonal microservice boundaries.

This skill specializes the broader `go-aws-lambda-microservice-hexagonal` skill and complements:
- `go-lambda-structured-logging`
- `go-lambda-error-handling`

This skill owns:
- tracer and meter provider setup
- resource and service identity
- span and metric boundaries
- correlation propagation across HTTP and SQS flows
- AWS SDK v2 client observability wiring

This skill does **not** own general application log design. Continue to use `go-lambda-structured-logging` for structured logs.

## Use this skill when

- the codebase is Go
- the compute model is AWS Lambda
- the task adds or modifies tracing, metrics, telemetry bootstrap, or context propagation
- the service needs portable telemetry that can be exported to AWS-native or third-party backends
- the task changes HTTP or SQS adapters and requires span or metric boundaries
- the task instruments AWS SDK v2 clients or other significant outbound calls

## Do not use this skill when

- the task is only about structured application logs
- the task is only about domain error taxonomy with no telemetry changes
- the function is a trivial throwaway Lambda where traces and metrics add no operational value
- the task is a broad observability-platform migration that already has an approved design outside this skill

## Core principles

- Treat OpenTelemetry as the instrumentation contract.
- Keep telemetry vendor-neutral at the application boundary.
- Keep provider setup centralized in bootstrap.
- Keep domain and application logic free from exporter and backend details.
- Instrument meaningful boundaries, not every line of execution.
- Preserve request or message correlation across inbound and outbound boundaries.
- Keep high-cardinality and sensitive attributes out of telemetry.

## Default implementation stance

- Prefer explicit OpenTelemetry SDK setup in the bootstrap/platform layer.
- Prefer one provider bootstrap for tracing and one for metrics.
- Prefer clear `service.name`, environment, and version resource attributes.
- Prefer manual instrumentation at inbound adapters, application use-case boundaries, and important outbound adapters.
- Prefer AWS SDK v2 observability adapters instead of hand-rolled per-call tracing hooks.
- Keep telemetry signal ownership narrow: this skill handles traces, metrics, and correlation, not general log formatting.

## Architecture rules

### Bootstrap / platform layer

- Build tracer and meter providers in one bootstrap location.
- Centralize resource attributes, exporter wiring, and shutdown hooks.
- Do not let packages instantiate their own providers ad hoc.
- Keep backend-specific endpoints, headers, and exporters in configuration/bootstrap code only.
- If ADOT Lambda layers or collectors are used, the application code must still keep provider ownership clear and explicit.

### Domain layer

- Domain code should not know about exporters, collectors, or telemetry backends.
- Domain code should not depend directly on AWS event types for telemetry.
- Domain instrumentation is acceptable only when the domain boundary itself is the meaningful span owner.
- Do not pollute entities or value objects with telemetry-only dependencies.

### Application layer

- Application handlers may create or annotate spans when a use case boundary is operationally meaningful.
- Application metrics should reflect meaningful business or operational outcomes, not incidental internal counters.
- Keep telemetry names aligned with use-case and bounded-context language.
- Avoid duplicate span creation in both application and adapter layers for the same boundary unless nesting is intentional.

### Inbound adapters

- Inbound adapters own extraction or creation of request/message correlation context.
- HTTP adapters should create or continue spans for the request boundary and record route/method/operation context.
- SQS adapters should create or continue spans for batch and per-record work when that distinction is operationally meaningful.
- Inbound adapters must not leak transport-specific event types into downstream telemetry APIs.

### Outbound adapters

- Outbound adapters own dependency spans and dependency-related metrics when those boundaries matter operationally.
- Use instrumentation helpers or SDK integrations where available instead of ad hoc span scattering.
- Record dependency latency, retries, throttling, and terminal failures where they are operationally meaningful.
- Do not capture sensitive payloads or unbounded attributes in dependency spans.

## Go-specific rules

- Prefer OpenTelemetry Go APIs and SDKs directly where instrumentation is needed.
- Keep tracer acquisition explicit and stable.
- Prefer semantic attribute names and stable instrument names over improvised conventions.
- Keep context propagation explicit through function boundaries that already accept `context.Context`.
- Do not treat `panic` recovery as a normal telemetry strategy.
- Do not create spans with highly variable names that destroy aggregation value.

## Lambda-specific rules

- Initialize provider wiring in bootstrap so warm invocations can reuse it.
- Preserve Lambda invocation context and request identifiers when correlating telemetry.
- Keep span and metric boundaries aligned with meaningful Lambda work units: request, batch, record, use case, dependency.
- Avoid excessive per-record telemetry in very high-volume SQS consumers unless the task explicitly requires it.
- Ensure provider shutdown/flush behavior is compatible with Lambda execution completion.

## Signal ownership rules

### Traces

- Create spans for meaningful inbound requests, use-case boundaries, and important outbound dependencies.
- Record failures and status meaningfully without duplicating the same error semantics in every nested span.
- Keep span names stable and low-cardinality.

### Metrics

- Capture duration, success/failure, retries, and other outcome-oriented measurements where operationally useful.
- Prefer metrics that answer operational questions such as “is it failing?”, “is it slow?”, or “is it retrying?”.
- Do not emit unbounded label/cardinality combinations.

### Logs

- This skill may attach trace or span correlation to logs when repository conventions require it.
- This skill does not replace the logging skill for application log format, redaction, or level policy.
- Do not force application logs through experimental OTel log paths unless the task explicitly requires it.

## HTTP mode rules

- Instrument the HTTP request boundary in the HTTP adapter.
- Keep HTTP route or operation names stable in span and metric dimensions.
- Record terminal failure status at the adapter or use-case boundary where it becomes meaningful.
- Do not use raw paths or user-specific values as high-cardinality span names or metric labels.

## SQS mode rules

- Treat SQS delivery as at-least-once and reflect retry-aware behavior in metrics and span status.
- Instrument batch-level behavior when batch outcome matters operationally.
- Instrument per-record behavior only when the added value justifies the volume.
- Preserve message correlation information where available.
- In FIFO flows, keep failure telemetry aligned with the stop-on-first-failure behavior when applicable.

## Correlation rules

- Keep the same conceptual unit of work joinable across traces, metrics, and structured logs.
- Preserve context propagation through application and outbound call boundaries.
- Use stable service identity and deployment metadata.
- Do not invent multiple aliases for the same correlation concept.

## Sensitive-data and cardinality rules

- Never record secrets, tokens, credentials, raw authorization headers, or full sensitive payloads in spans, metrics, or OTel log bridges.
- Avoid unbounded identifiers as metric labels.
- Avoid span attributes whose value space explodes with user, payload, or raw request variance.
- Prefer stable operation names over dynamic names.

## Testing rules

- Unit tests for provider bootstrap should verify stable construction and configuration boundaries where contractual.
- Adapter tests should verify context propagation and boundary instrumentation behavior where changed behavior is part of the contract.
- Tests may use in-memory or test exporters instead of real backends.
- Do not require live vendor backends for ordinary unit tests.
- Integration tests for collectors, OTLP endpoints, or vendor export should be explicit and separate from fast unit tests.

## Dependency rules

- Prefer OpenTelemetry Go SDK/API packages directly for tracing and metrics.
- Prefer AWS-supported integrations for Lambda and AWS SDK v2 observability when they solve a real need.
- Any additional observability framework, wrapper, or custom facade requires explicit approval from the active prompt, task, or repository conventions.
- Do not introduce backend-specific instrumentation into domain or application code.

## Checklist for implementation

- Centralize tracer and meter provider bootstrap.
- Set stable resource attributes such as service name, environment, and version when repository conventions require them.
- Instrument inbound HTTP or SQS boundaries intentionally.
- Instrument important outbound AWS SDK v2 or dependency calls.
- Keep trace and metric names stable and low-cardinality.
- Keep backend/exporter specifics out of domain and application layers.
- Add or update tests for propagation and instrumentation behavior where contractual.

## Done when

- telemetry bootstrap is centralized
- traces and metrics are emitted at meaningful boundaries
- HTTP and SQS flows preserve useful correlation context
- AWS SDK v2 clients or other important dependencies are instrumented where operationally relevant
- signal names and attributes are stable and low-cardinality
- telemetry does not leak secrets or sensitive payloads
- tests cover changed propagation or provider behavior where applicable
