# Architecture guidance

This skill assumes the service follows the broader `go-aws-lambda-microservice-hexagonal` structure.

## Where logging belongs

### Bootstrap / platform
Owns:
- base logger construction
- output format
- common static fields
- default level
- environment/service metadata

### Inbound adapters
Own:
- request/message correlation
- trigger-specific enrichment
- terminal boundary failure logging
- optional start/finish logs when operationally valuable

### Application layer
Owns:
- decision/outcome logs only when the application boundary is the right owner
- operation fields that describe use-case execution

### Domain layer
Usually does not log directly.

### Outbound adapters
Own:
- dependency retry logs
- throttling/degraded behavior logs
- terminal dependency failure logs when the dependency boundary is the right owner

## Recommended field model

Common:
- service
- operation
- trigger
- request_id
- message_id
- outcome
- error

HTTP-oriented:
- method
- route
- status_code

SQS-oriented:
- queue_arn
- receive_count
- batch_size

Keep the model small and stable. If a field is not consistently present or useful, do not force it into every log line.

## Logger ownership

Use one base logger created during bootstrap.
Derive enriched loggers at boundaries using `With(...)` or equivalent context-enrichment patterns.

Avoid:
- package-level ad hoc logger configuration
- one logger implementation per package
- multiple unrelated field naming conventions
