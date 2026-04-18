# Architecture Guidance

## Intent

This skill organizes a Go Lambda microservice around clear business boundaries.

- **Domain** holds entities, value objects, and domain rules.
- **Application** orchestrates use cases.
- **Ports** define boundaries that the application depends on.
- **Inbound adapters** translate API Gateway HTTP API or SQS events into application inputs.
- **Outbound adapters** implement repositories, publishers, and external integrations.
- **Platform** wires config, logging, metrics, tracing, and SDK clients.

## Preferred package map

```text
cmd/
  http-lambda/
  sqs-lambda/
internal/
  domain/
    <bounded_context>/
  application/
    <use_case>/
  ports/
    inbound/
    outbound/
  adapters/
    inbound/
      httpapi/
      sqs/
    outbound/
      dynamodb/
      s3/
      sns/
      eventbridge/
      httpclient/
  platform/
    bootstrap/
    config/
    logging/
    observability/
```

## Dependency direction

- Domain depends on nothing outward.
- Application depends on domain and ports.
- Adapters depend on ports and platform concerns.
- Platform may depend on SDKs and runtime-specific packages.
- Lambda entrypoints depend on platform bootstrap and inbound adapters.

## Boundary rules

- Do not use API Gateway or SQS event types in domain or application packages.
- Do not expose SDK response/request models from ports.
- Do not hide transport-specific behavior inside domain entities.
- Do not let repositories or publishers bypass ports and write directly from handlers.

## Trigger guidance

### HTTP API

Use for synchronous request/response behavior.
Keep:
- request parsing
- validation at the edge
- response mapping at the edge

### SQS

Use for asynchronous commands/events.
Keep:
- message parsing in the SQS adapter
- idempotency at the application boundary or persistence boundary
- partial batch response assembly in the SQS adapter
