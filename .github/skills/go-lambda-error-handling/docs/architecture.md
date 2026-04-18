# Architecture Notes — go-lambda-error-handling

This skill assumes a hexagonal Lambda microservice.

## Error ownership by layer

- **Domain** owns business-rule and invariant errors.
- **Application** owns use-case orchestration and can add operation context or retryability classification.
- **Outbound adapters** translate foreign SDK/client failures into port-level failures while preserving causes.
- **Inbound adapters** own final mapping to transport behavior:
  - HTTP status code + safe response body
  - SQS batch item failure list + retry semantics

## Boundary rule

A lower layer returns errors upward.
A boundary layer converts errors into transport-specific behavior.
No lower layer should know how HTTP or SQS expresses failure.

## Minimum error taxonomy

Keep this small. Introduce only the categories the service really needs.

Examples:
- invalid input
- not found
- conflict / invariant violation
- unauthorized / forbidden
- dependency unavailable / timeout / throttled
- unexpected internal failure
- retryable message failure
- terminal message failure

Avoid giant framework-style taxonomies unless the service genuinely needs them.

## Panic stance

Go and Lambda both allow panics, but this skill treats them as exceptional.

Allowed uses:
- impossible states
- programmer bugs discovered during execution
- optional narrow edge recovery to convert panic into a terminal boundary failure with diagnostics

Disallowed uses:
- validation failures
- dependency failures
- missing records
- expected business rule outcomes
- batch item classification
