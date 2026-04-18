---
name: "go-lambda-error-handling"
description: "A skill for handling errors in Go microservices on AWS Lambda."
user-invocable: false
---

# SKILL: go-lambda-error-handling

## Purpose

Implement, modify, or review error handling in Go AWS Lambda services so that failures are explicit, diagnosable, and mapped correctly at architecture and transport boundaries.

This skill specializes the broader `go-aws-lambda-microservice-hexagonal` skill and applies to services whose inbound trigger is one or both of:
- API Gateway HTTP API
- Amazon SQS

## Use this skill when

- the codebase is Go
- the compute model is AWS Lambda
- the service uses or should use explicit errors instead of hidden fallbacks
- the task touches domain/application/adapters boundaries
- the service needs correct error mapping for HTTP or SQS behavior
- the service needs retry-aware, idempotency-aware failure semantics

## Do not use this skill when

- the task is only about logging or telemetry with no change to error behavior
- the task is only about styling or refactoring with no error-path impact
- the service is a trivial one-off function with no reusable domain/application boundary

## Core principles

- Treat errors as values and handle them explicitly.
- Prefer returning errors over using panic for ordinary control flow.
- Wrap errors with context using `%w` so callers can inspect them with `errors.Is` and `errors.As`.
- Keep domain, validation, dependency, and unexpected failures distinguishable.
- Convert errors only at meaningful boundaries.
- Do not let AWS transport types or HTTP concerns leak into domain/application errors.
- Do not swallow errors, invent fallback success values, or return misleading partial success.
- Panic is reserved for unrecoverable programmer bugs or truly impossible states, not normal business failure.

## Architecture rules

### Domain layer

- Domain code may define sentinel or typed domain errors when callers need stable classification.
- Domain errors must not depend on AWS, HTTP, SQS, or infrastructure-specific types.
- Domain errors should represent business facts, invariants, and rule violations.
- Do not log in domain code as a substitute for returning an error.

### Application layer

- Application handlers return explicit errors to the inbound adapter.
- Application code may wrap lower-level failures with operation context.
- Application code may classify retryable vs terminal failures when the transport requires it.
- Application code must not return API Gateway response types or SQS batch response types.

### Outbound adapters

- Outbound adapters translate foreign SDK/client failures into repository/client/publisher errors meaningful to the application layer.
- Preserve the original cause when translating with `%w`.
- Do not leak low-level transport payloads or secrets through returned error text.
- Keep provider-specific branching confined to the adapter boundary.

### Inbound adapters

- Inbound adapters own final error mapping to HTTP or SQS behavior.
- Inbound adapters may convert application/domain errors into transport-specific responses.
- Inbound adapters must not push HTTP or SQS semantics into domain/application code.

## Go-specific rules

- Prefer `fmt.Errorf("context: %w", err)` for wrapping.
- Prefer `errors.Is` and `errors.As` for inspection.
- Do not compare wrapped errors by string.
- Do not use panic/recover as a replacement for ordinary error returns.
- If panic recovery exists at the Lambda edge, it must be narrow, explicit, and convert the panic into a terminal failure path plus diagnostic logging.
- Error strings should be lowercase and should not end with punctuation.

## HTTP mode rules

- For expected application failures, return a valid HTTP response with the correct status code from the HTTP adapter.
- Do not bubble expected business or validation failures as raw Lambda invocation errors.
- If a Lambda function returns an error or an invalid proxy response, API Gateway can return a generic internal error to the client; avoid this for expected error cases.
- Keep HTTP status mapping in the HTTP adapter only.
- Distinguish at least these classes when relevant:
  - validation / bad input -> 4xx
  - not found -> 404
  - conflict / invariant violation -> 409 or other contract-appropriate 4xx
  - dependency or unexpected failure -> 5xx
- External error responses must be safe; they must not expose secrets, stack traces, or raw infrastructure details.

## SQS mode rules

- Treat delivery as at-least-once.
- Make message handling idempotent.
- Classify failures into retryable vs terminal where the business and infrastructure behavior require it.
- When partial batch response is enabled, catch per-record failures and return `batchItemFailures` for failed records.
- Do not throw from the handler after partial batch mode is adopted; an unhandled exception makes the entire batch fail.
- For FIFO queues, stop processing after the first failure and return failed plus unprocessed message IDs in `batchItemFailures` to preserve ordering.
- Keep batch transport behavior inside the SQS adapter.

## Retry and idempotency rules

- Retryability must be a deliberate classification, not an accidental side effect of generic failure handling.
- Retryable errors should be reserved for transient dependency/network/throttling conditions or explicitly retryable workflows.
- Terminal errors should not be retried forever; prefer DLQ/redrive strategy where applicable.
- Idempotency must protect business side effects when the same message or command is processed more than once.

## Logging interaction rules

- Do not log and return the same error at every layer.
- Log terminal failures at the boundary where they become operationally meaningful.
- It is acceptable to add context while propagating without logging at every step.
- Error handling must remain correct even if logging is removed.

## Testing rules

- Unit tests for domain/application code must assert error classification with `errors.Is` / `errors.As` where appropriate.
- Adapter tests must verify error mapping behavior, not only happy paths.
- HTTP adapter tests must verify expected status codes and safe response bodies for error scenarios.
- SQS adapter tests must verify partial batch behavior, retry classification, and FIFO stop-on-first-failure behavior when relevant.
- Panic recovery tests, if implemented, must verify narrow edge recovery only.

## Dependency rules

- Prefer Go standard library error facilities first.
- Any third-party error package requires explicit approval from the active prompt, task, or repository conventions.
- Do not introduce convenience libraries that replace ordinary Go error returns unless explicitly required.

## Checklist for implementation

- Define stable domain/application error categories only where callers need them.
- Preserve causes with `%w` when crossing boundaries.
- Keep transport mapping inside inbound adapters.
- Keep provider/client translation inside outbound adapters.
- Ensure HTTP expected failures become valid HTTP responses.
- Ensure SQS expected per-record failures become correct partial batch responses.
- Keep panic/recover out of ordinary control flow.
- Add or update tests for changed error paths.

## Done when

- the service uses explicit, inspectable error values
- wrapped errors preserve useful cause information
- domain/application errors are transport-agnostic
- HTTP adapters map expected failures to correct responses without raw Lambda errors
- SQS adapters implement explicit retry/failure behavior and partial batch response where applicable
- tests verify error classification and boundary mapping
- no hidden fallbacks, swallowed errors, or string-based error branching remain in touched areas
