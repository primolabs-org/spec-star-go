# Task Routing

## Choose HTTP mode when

- the caller expects a synchronous response
- the contract is naturally request/response
- status codes and response bodies matter to callers

## Choose SQS mode when

- delivery is asynchronous
- retries are expected
- the service reacts to commands or events
- processing latency is decoupled from the caller

## Choose shared-core work when

- the task adds or changes domain rules
- the task adds a new use case
- the task changes repository or publisher contracts
- the task affects behavior shared by both HTTP and SQS adapters

## Skill operating steps

1. Classify the task as `http`, `sqs`, or `shared-core`.
2. Identify the affected bounded context and use case.
3. Touch only the domain, application, ports, adapters, and platform files required by that change.
4. Keep transport-specific logic in the inbound adapter.
5. Keep external-service specifics in outbound adapters.
6. Add tests that match the changed surface.
