# Task routing

Use this skill for tasks such as:
- introduce structured JSON logging to a Go Lambda
- migrate ad hoc `log.Printf` usage to `slog`
- add request ID or message ID correlation fields
- standardize log level usage
- remove noisy duplicate logs
- ensure HTTP and SQS adapters log meaningful terminal failures

Do not use this skill as the primary skill when the task is mainly about:
- error taxonomy or boundary error mapping with no logging contract change
- metrics and tracing design
- IaC-only changes unrelated to application logging behavior

## Good pairings

Pair with:
- `go-aws-lambda-microservice-hexagonal`
- `go-lambda-error-handling`
- future `go-lambda-observability`

## Prompt hints

Useful task prompt additions:
- allowed dependencies
- whether JSON output is mandatory
- expected field names
- whether debug logs are allowed
- whether payload logging is forbidden or narrowly allowed
- whether log snapshots/golden tests are part of repo conventions
