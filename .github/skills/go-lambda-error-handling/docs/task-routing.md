# Task Routing — go-lambda-error-handling

Use this skill for tasks like:

- Introduce stable domain or application errors in a Go Lambda service.
- Replace string-based error branching with errors.Is / errors.As.
- Wrap outbound dependency errors with context using %w.
- Map application errors to HTTP responses in an API Gateway adapter.
- Implement retryable vs terminal classification for SQS processing.
- Implement partial batch failure handling without failing whole successful records.
- Add or tighten edge panic recovery in a Lambda handler.
- Add tests for HTTP or SQS error scenarios.

Do not use this skill alone for:

- pure logging design
- pure observability instrumentation
- generic cleanup unrelated to error behavior
- broad architecture redesign outside Lambda error boundaries
