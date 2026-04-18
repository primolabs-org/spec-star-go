---
name: "go-aws-lambda-microservice-hexagonal"
description: "A skill for building Go microservices on AWS Lambda using hexagonal architecture."
user-invocable: false
---

# SKILL: go-aws-lambda-microservice-hexagonal

## Purpose

Build or modify a Go microservice whose compute boundary is AWS Lambda and whose internal structure follows hexagonal architecture.

This skill supports one or both inbound trigger types:

- API Gateway HTTP API
- Amazon SQS

Use this skill when the service must behave like a bounded microservice instead of a one-off function and when business logic should remain reusable across multiple protocols.

## Use this skill when

- the codebase is in Go
- the runtime target is AWS Lambda
- the service has meaningful business/domain behavior
- the implementation needs clear domain, application, ports, and adapters boundaries
- the service is triggered by API Gateway HTTP API, SQS, or both

## Do not use this skill when

- the task is a tiny one-off Lambda script with no meaningful domain boundary
- the service intentionally embeds a heavy web framework inside Lambda
- the task does not benefit from transport/infrastructure separation
- the task is primarily infrastructure provisioning with little or no service logic

## Architecture rules

- Treat Lambda as the compute host, not as the business architecture.
- Keep domain logic in `internal/domain`.
- Keep application use cases in `internal/application`.
- Keep inbound and outbound contracts in `internal/ports`.
- Keep transport and infrastructure code in `internal/adapters`.
- Keep bootstrap, config, logging, and observability wiring in `internal/platform`.
- Keep AWS event types confined to inbound adapters.
- Keep AWS SDK clients and third-party integrations confined to outbound adapters.
- Keep handlers thin: map input, call the use case, map the result.
- Do not let domain/application code depend on API Gateway events, SQS events, DynamoDB types, or SDK response types.

## Runtime rules

- Prefer `provided.al2023`.
- Do not use Lambda layers for Go application dependencies.
- Prefer a single compiled binary and minimal third-party dependencies.
- Initialize reusable clients, configuration, and wiring outside the handler when safe.
- Keep cold-start-sensitive initialization small and intentional.

## Dependency rules

- Prefer the Go standard library unless a third-party dependency provides clear, concrete value.
- Prefer AWS-supported Go packages for Lambda and AWS integrations.
- Do not introduce new dependencies for convenience alone.
- Any new logging, testing, mocking, HTTP, or observability library must be explicitly allowed by the active prompt, task, or repository conventions.
- Keep the dependency surface minimal and easy to justify in review.

## Trigger mode rules

### HTTP mode

- Prefer API Gateway HTTP API by default.
- Keep request/response mapping inside the HTTP inbound adapter.
- Keep validation and transport translation at the boundary.
- Return application results from use cases, not transport-specific DTOs.
- Map application errors to HTTP responses only in the inbound adapter.

### SQS mode

- Treat delivery as at-least-once.
- Make consumers idempotent.
- Process records independently where possible.
- Implement partial batch response.
- Distinguish retryable failures from terminal failures.
- Keep message parsing and batch response assembly inside the SQS inbound adapter.

## Logging and observability rules

- Use structured logs.
- Include correlation identifiers when available.
- Emit metrics for request/message count, success, failure, latency, and retries.
- Add tracing when the service has important external dependencies or participates in distributed flows.
- Never log secrets or full sensitive payloads.

## Testing rules

- Domain and application layers must be unit tested without AWS event fixtures.
- Inbound adapters must be tested for mapping correctness.
- Outbound adapters must be tested for contract correctness where practical.
- HTTP mode must include request/response mapping tests.
- SQS mode must include batch failure behavior and idempotency-sensitive tests.

## Test placement rules

- Put unit tests beside the code they verify.
- Test files must use the `*_test.go` naming convention.
- Prefer tests in the same package as the code under test.
- Use a corresponding `_test` package only when black-box testing of exported behavior is intentional.
- Do not place ordinary package unit tests in a global `/tests` directory.
- Reserve `test/integration` and `test/contract` for cross-package, environment-backed, or boundary-level tests.
- Keep domain and application tests free of AWS event fixtures.
- Keep HTTP and SQS adapter tests close to their adapter packages.
- Integration tests that require AWS, LocalStack, Docker, or real infrastructure must be explicitly marked and must not run implicitly as ordinary fast unit tests.

## Expected layout

```text
cmd/
  http-lambda/
    main.go
  sqs-lambda/
    main.go

internal/
  domain/
  application/
  ports/
  adapters/
    inbound/
    outbound/
  platform/
```

## Working rules for the agent

When using this skill:

1. Identify whether the task is HTTP mode, SQS mode, or shared core logic.
2. Keep the active change scoped to the relevant domain, application use case, and adapters.
3. Avoid introducing framework-heavy abstractions unless the task explicitly requires them.
4. Prefer explicit ports and adapters over leaking transport or SDK types inward.
5. Add or update tests for any changed behavior in domain, application, or trigger adapters.
6. Leave behind a short architecture note when introducing a new use case, port, or adapter.

## Done when

- the handler is thin
- domain/application code is transport-agnostic
- required inbound adapter exists for HTTP or SQS
- outbound dependencies are isolated behind ports/adapters
- bootstrap and dependency wiring are centralized
- logging/metrics/tracing are present where operationally relevant
- tests cover changed domain/application logic and trigger-specific adapter behavior
