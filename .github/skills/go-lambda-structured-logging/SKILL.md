---
name: "go-lambda-structured-logging"
description: "A skill for implementing structured logging in Go microservices on AWS Lambda."
user-invocable: false
---

# SKILL: go-lambda-structured-logging

## Purpose

Implement, modify, or review structured logging in Go AWS Lambda services so logs are machine-parseable, correlated, low-noise, and aligned with hexagonal microservice boundaries.

This skill specializes the broader `go-aws-lambda-microservice-hexagonal` skill and applies to services whose inbound trigger is one or both of:
- API Gateway HTTP API
- Amazon SQS

## Use this skill when

- the codebase is Go
- the compute model is AWS Lambda
- the task changes logging behavior, logger construction, log fields, or log level policy
- the task introduces or modifies HTTP or SQS inbound adapters
- the task needs consistent request or message correlation fields
- the service needs structured logs suitable for CloudWatch filtering and operational debugging

## Do not use this skill when

- the task is only about error classification with no logging changes
- the task is only about metrics or tracing with no logging changes
- the service is a trivial throwaway function where structured operational logs are unnecessary
- the task is a repository-wide logging framework migration without an approved design

## Core principles

- Prefer structured logs over ad hoc string formatting.
- Prefer a single logger bootstrap and field model over package-local logger drift.
- Keep logs aligned with architecture boundaries: inbound adapter, application decision, outbound dependency, terminal failure.
- Keep logs low-noise: log meaningful transitions, failures, retries, and outcomes, not every line of execution.
- Use logs to explain what happened, not to replace correct error handling or metrics.
- Keep secrets, credentials, tokens, and raw sensitive payloads out of logs.
- Keep field names stable and consistent across handlers and triggers.

## Default implementation stance

- Prefer the Go standard library `log/slog` package for structured logging.
- Prefer JSON output for production Lambda functions.
- Initialize the logger during bootstrap and reuse it across warm invocations.
- Carry correlation fields through context or explicit logger enrichment at adapter boundaries.
- Keep logging vendor-neutral unless the active task explicitly requires a different library.

## Architecture rules

### Bootstrap / platform layer

- Create the base logger in one bootstrap location.
- Configure handler, level, common service fields, and environment fields centrally.
- Do not let each package invent its own logging format or global logger behavior.
- If Lambda advanced logging controls are used, keep application logger output compatible with JSON log filtering.

### Domain layer

- Domain code should generally avoid direct logging.
- Domain invariants and failures should be returned to callers, not logged inside domain code.
- Domain logging is acceptable only when explicitly required by the design and the signal cannot be emitted meaningfully at a higher boundary.

### Application layer

- Log meaningful decisions and terminal outcomes only when the application layer is the correct owner of that context.
- Avoid logging every wrapped error on the way up the stack.
- Prefer structured fields that describe the operation, entity, and decision boundary.

### Inbound adapters

- Inbound adapters own request/message correlation enrichment.
- HTTP adapters should attach request-oriented fields such as request ID, route, method, and operation where available.
- SQS adapters should attach message-oriented fields such as message ID, queue or source ARN, receive count when available, and operation.
- Inbound adapters should log request or message start only when that event is operationally useful and not excessively noisy.
- Inbound adapters should own terminal failure logging for client-facing HTTP failures or message-processing failures.

### Outbound adapters

- Outbound adapters may log dependency calls, retries, throttling, and terminal failures when that boundary is operationally meaningful.
- Do not dump full dependency payloads or raw sensitive values.
- Keep one clear owner for terminal dependency failure logs where possible.

## Go-specific rules

- Prefer `log/slog` with key-value attributes.
- Prefer `logger.With(...)` or context-aware enrichment over repeated manual field concatenation.
- Keep field keys stable, lowercase, and semantically clear.
- Prefer explicit attribute names over generic `data`, `details`, or `payload`.
- Do not build structured logs by manually assembling JSON strings.
- Do not rely on log message text alone for filtering when stable fields can carry the same meaning.

## Lambda-specific rules

- Prefer JSON log output for Lambda functions in production.
- Keep log entries compatible with Lambda JSON log filtering when advanced logging controls are enabled.
- Preserve and log the Lambda request ID where available.
- Reuse the logger across warm invocations instead of rebuilding it on every request.
- Do not depend on Lambda-specific runtime side effects for business-critical fields; attach required fields explicitly in code.

## Field model rules

At a minimum, keep these conceptual fields consistent where they exist:
- `service`
- `operation`
- `request_id`
- `message_id`
- `trigger`
- `level`
- `error`
- `outcome`
- `cold_start` when the service tracks it
- environment or deployment identity fields when repository conventions require them

Rules:
- Use `request_id` for invocation/request correlation, not multiple aliases for the same concept.
- Use `message_id` for per-message SQS correlation.
- Use `trigger` to distinguish `http` and `sqs` flows when both exist in the same service.
- Use explicit fields for identifiers instead of embedding them only in message text.

## Level rules

- `DEBUG` is for deep diagnostic detail that is normally disabled in production.
- `INFO` is for meaningful lifecycle and business-relevant transitions that are not failures.
- `WARN` is for degraded or unusual behavior that is not terminal.
- `ERROR` is for terminal operation failures or dependency failures that matter operationally.
- Do not log expected validation failures as infrastructure errors unless the contract or operations model requires it.
- Do not emit `ERROR` for normal retry attempts unless the retry step itself is terminally failing.

## HTTP mode rules

- Log terminal HTTP failures at the HTTP adapter boundary with route or operation context.
- Keep client-safe response mapping separate from internal diagnostic detail.
- Do not log the entire request body by default.
- Log enough context to distinguish invalid input, not-found, conflict, dependency failure, and unexpected failure classes when those distinctions matter operationally.

## SQS mode rules

- Log per-record failures with `message_id` and operation context.
- Keep batch-level summary logs separate from per-record failure logs.
- In partial batch mode, avoid a single ambiguous batch failure log that hides which record failed.
- For FIFO queues, if processing stops after the first failure, log the first failure and the reason subsequent records were not processed.
- Do not log every successful message in a high-throughput consumer unless explicitly required.

## Sensitive-data rules

- Never log secrets, credentials, tokens, session material, or full authorization headers.
- Never log full raw events or payloads by default.
- Redact or omit sensitive business identifiers when repository conventions require it.
- If the task explicitly requires payload logging, keep it narrow, justified, and safe.

## Testing rules

- Unit tests for logging helpers should verify stable field presence and level behavior where the helper is part of the contract.
- Adapter tests should verify correlation enrichment for HTTP and SQS paths.
- Tests may assert structured output using handler-backed buffers instead of brittle string matching on free-form log lines.
- Do not over-test incidental message wording when field structure is the true contract.
- If the repository uses golden or snapshot tests for logs, keep them narrow and stable.

## Dependency rules

- Prefer Go standard library logging first.
- Any third-party logging package requires explicit approval from the active prompt, task, or repository conventions.
- Do not introduce broad logging frameworks for convenience alone.
- Do not introduce Lambda-specific logging helpers that duplicate straightforward `slog` behavior unless the task explicitly requires them.

## Checklist for implementation

- Centralize logger bootstrap.
- Use structured JSON logging for Lambda production paths.
- Attach stable common fields such as service and operation.
- Attach request or message correlation fields in inbound adapters.
- Keep terminal failure logging at meaningful boundaries.
- Remove temporary debug logging when the task is complete.
- Add or update tests for logging helpers or boundary enrichment where changed behavior is contractual.

## Done when

- the service emits structured logs with stable fields
- logger construction is centralized and reused across invocations
- HTTP and SQS paths attach useful correlation fields
- log levels are used intentionally
- logs do not leak secrets or full sensitive payloads
- noisy duplicate logging has been removed from touched areas
- tests cover changed logging helpers or adapter enrichment where applicable
