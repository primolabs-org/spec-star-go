# Done-When Checklist

## General

- [ ] The change is scoped to the active use case and bounded context.
- [ ] Domain and application code remain independent from AWS event types.
- [ ] Outbound integrations remain behind ports/adapters.
- [ ] Touched areas are clean and aligned with current behavior.

## HTTP mode

- [ ] The HTTP Lambda handler is thin.
- [ ] API Gateway request parsing stays in the HTTP adapter.
- [ ] HTTP response mapping stays in the HTTP adapter.
- [ ] Application results are transport-agnostic.
- [ ] Error mapping to HTTP status/body happens only at the boundary.

## SQS mode

- [ ] The SQS Lambda handler is thin.
- [ ] Message parsing stays in the SQS adapter.
- [ ] Consumer behavior is idempotent.
- [ ] Partial batch response is implemented when processing batches.
- [ ] Retryable and terminal failures are handled intentionally.

## Observability

- [ ] Structured logging is present on important boundaries.
- [ ] Correlation identifiers are preserved when available.
- [ ] Metrics exist for critical success/failure/latency signals.
- [ ] Tracing is added when the flow crosses meaningful dependencies.

## Tests

- [ ] Domain/application behavior is covered by focused tests.
- [ ] Adapter mapping behavior is covered where the task changes it.
- [ ] SQS batch failure behavior is tested when relevant.
- [ ] Regressions introduced by the change are covered.
