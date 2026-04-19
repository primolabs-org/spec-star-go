# AGENTS.md

This file defines project-specific guidance for coding agents working in this repository.

---

## 1) Repository purpose

- This repository implements a Go-based AWS Lambda microservice for fixed-income wallet position management.
- Inbound protocols may include API Gateway HTTP API and Amazon SQS.
- The service follows hexagonal architecture so business logic remains isolated from AWS transport and infrastructure details.

---

## 2) Core expectations

- Respect existing architecture and repository conventions.
- Prefer minimal, high-confidence changes over broad rewrites.
- Keep touched areas in a cleaner state than you found them.
- Do not introduce speculative abstractions or convenience dependencies without clear need.
- Do not invent business behavior when requirements are ambiguous.
- Do not leave behind dead code, stale comments, temporary debug code, or broken tests.
- Do not claim work is complete until required validation commands have passed, or you explicitly state what could not be validated.

---

## 3) Skill routing

When the relevant skills are installed for this project, use them.

### Base architecture skill

Use `go-aws-lambda-microservice-hexagonal` when:
- the task modifies a Go AWS Lambda service
- the service is treated like a microservice
- inbound flows are HTTP, SQS, or both
- domain/application logic must remain isolated from AWS-specific concerns

### Cross-cutting Go/Lambda skills

Use `go-lambda-error-handling` when:
- modifying error classification, propagation, wrapping, or boundary mapping
- changing HTTP error responses
- changing SQS retry, partial batch, idempotency, or failure behavior

Use `go-lambda-structured-logging` when:
- adding or changing logs
- introducing request, message, or workflow correlation fields
- changing logging format or operational logging boundaries

Use `go-lambda-observability-otel` when:
- adding or changing tracing or metrics
- instrumenting AWS SDK v2 calls
- changing span boundaries, metric names, or telemetry attributes
- modifying observability bootstrap or OTel provider wiring

### Cross-cutting instructions

Always follow repository instructions for:
- cleanup
- fail-fast behavior
- testing
- error handling
- logging
- observability

If the repository provides both instructions and skills, use instructions as standing policy and skills as practical implementation guidance.

---

## 4) Architectural boundaries

- Treat AWS Lambda as the compute host, not the business architecture.
- Keep Lambda handlers thin.
- Keep AWS event types, Lambda context details, and transport DTOs confined to inbound adapters.
- Keep domain and application layers free of API Gateway, SQS, CloudWatch, and AWS SDK event/transport types.
- Keep AWS SDK clients and external integrations behind outbound adapters/ports.
- Prefer explicit use-case inputs and outputs over leaking infrastructure types across layers.

### Inbound rules

For HTTP:
- keep request parsing and HTTP response mapping in inbound adapters
- do not return raw API Gateway-specific contracts from domain/application layers

For SQS:
- keep message parsing and batch response mapping in inbound adapters
- do not let domain/application code depend on `events.SQSEvent` or batch response types
- preserve idempotent processing assumptions

---

## 5) Dependency policy

- Prefer the Go standard library unless a third-party dependency provides clear and concrete value.
- Prefer AWS-supported Go libraries for Lambda and AWS integrations.
- Do not introduce a new dependency for convenience alone.
- Do not introduce heavy web frameworks, DI frameworks, or alternative logging/testing stacks unless the task or repository explicitly requires them.
- Treat logging, observability, mocking, and assertion libraries as opt-in, not default.
- If a new dependency is introduced, explain why the standard library or existing project dependencies were insufficient.

Optional project policy:
- allowed by default: `github.com/aws/aws-lambda-go`, `github.com/aws/aws-sdk-go-v2`, OpenTelemetry packages already adopted by the repo
- require explicit approval for: new HTTP frameworks, new logging libraries, new mocking frameworks, new observability frameworks

---

## 6) Deterministic quality gates

Before considering a task complete, run the relevant validation commands for the changed scope.

### Required defaults for Go changes

Run these unless the task explicitly states otherwise:

```bash
go test ./...
go vet ./...
golangci-lint run ./...
go build ./...
```

If formatting is needed, apply formatting before finalizing changes.

Preferred formatting options:

```bash
go fmt ./...
```

or, if this repository standardizes on golangci-lint formatters:

```bash
golangci-lint fmt
```

### Validation policy

- Do not skip validation because the change looks small.
- Do not rely on reasoning alone when automated validation is available.
- If a command fails, either fix the issue or report the exact failure clearly.
- Do not hide failing validation behind partial success language.
- If a command is too expensive or depends on unavailable infrastructure, state that explicitly and run the highest-confidence subset available.

### Changed-scope rule

At minimum, validate the packages directly affected by the change. When feasible and not prohibitively expensive, validate the full module.

---

## 7) Formatting, linting, and static analysis expectations

- Leave no malformed Go files behind.
- Leave no unresolved lint violations in touched areas unless the task explicitly allows a narrow exception.
- Remove dead code, unused imports, stale comments, and temporary debug artifacts exposed by the task.
- Keep names, comments, and package structure aligned with the current implementation.
- Do not silence static analysis findings without fixing the underlying issue or clearly documenting an approved exception.

Use the enabled `golangci-lint` linters as the primary static analysis gate: 

- errcheck: Errcheck is a program for checking for unchecked errors in Go code. These unchecked errors can be critical bugs in some cases.
- govet: Vet examines Go source code and reports suspicious constructs. It is roughly the same as 'go vet' and uses its passes. [auto-fix]
- ineffassign: Detects when assignments to existing variables are not used. [fast]
- staticcheck: It's the set of rules from staticcheck. [auto-fix]
- unused: Checks Go code for unused constants, variables, functions and types.

This repository requires a `.golangci.yml` at the root. Follow its configuration rather than inventing local conventions.

---

## 8) Error-handling rules

- Prefer explicit errors over hidden recovery.
- Use Go error wrapping and inspection consistently.
- Preserve causal chains when propagating failures.
- Do not use panic for routine business control flow.
- Do not swallow errors.
- Do not log an error and then pretend the operation succeeded.
- Map expected business failures at the transport boundary rather than leaking internal failure details.

For HTTP adapters:
- return valid, explicit boundary-safe HTTP responses
- do not rely on raw Lambda failures for expected application behavior

For SQS adapters:
- assume at-least-once delivery
- preserve idempotent behavior
- implement partial batch failure behavior when required by the service design

---

## 9) Logging rules

- Use structured logging.
- Prefer stable keys and contextual fields over decorative prose.
- Include correlation identifiers when available.
- Do not log secrets, credentials, tokens, or sensitive payloads.
- Do not duplicate the same terminal failure log across layers unless each log adds distinct operational value.
- Keep operational logs at meaningful boundaries: request start/end, message processing, retry, dependency failure, terminal outcome.

---

## 10) Observability rules

- Instrument critical paths so latency, throughput, retries, and failures are diagnosable.
- Keep observability changes aligned with the existing telemetry model in this repository.
- If this service uses OpenTelemetry, preserve resource identity, metric naming discipline, and span boundaries.
- Avoid high-cardinality attributes unless explicitly justified.
- Keep logs, metrics, and traces conceptually correlated around the same unit of work.

---

## 11) Testing rules

- Put unit tests beside the code they verify.
- Name Go test files with `*_test.go`.
- Prefer same-package tests by default.
- Use `_test` packages intentionally for black-box testing only.
- Reserve top-level `test/integration` or `test/contract` areas for cross-package or environment-backed tests.
- Keep domain and application tests free of AWS transport fixtures where possible.
- Keep adapter tests close to the adapter packages they verify.
- For bug fixes, add or update a regression test that would fail without the fix.

---

## 12) Completion contract

Do not report completion until you have done all of the following:

1. implemented the requested change within the approved scope
2. applied relevant skills and repository instructions
3. removed dead code or stale artifacts exposed by the change
4. updated or added tests for changed behavior
5. run the relevant validation commands
6. verified formatting and linting expectations
7. summarized exactly what changed, what was validated, and any remaining limitations

### Required final report format

At the end of the task, report:
- files changed
- behavior changed
- tests added or updated
- commands run
- command results
- any unresolved risks or deferred items

Do not say “done” without validation evidence.

---

## 13) Project-specific configuration

- Go toolchain: **1.26.2** (pinned in `go.mod`)
- Builder image: `golang:1.26.2-bookworm`
- Module/package entrypoints:
  - `cmd/http-lambda`
- Architecture overview:
  - Inbound adapters: `internal/adapters/inbound/httphandler`
  - Outbound adapters: `internal/adapters/outbound/postgres`
  - Bootstrap location: `cmd/http-lambda/main.go`
- Required commands:
  - Test: `go test ./...`
  - Lint: `golangci-lint run ./...`
  - Build: `go build ./...`
  - Vet: `go vet ./...`
- Approved direct dependencies:
  - `github.com/aws/aws-lambda-go`
  - `github.com/google/uuid`
  - `github.com/jackc/pgx/v5`
  - `github.com/shopspring/decimal`
- Infrastructure:
  - API Gateway HTTP API
  - Local test harness: Docker Compose + PostgreSQL + RIE-based local Lambda execution
  - PostgreSQL must define a non-empty `POSTGRES_PASSWORD`
  - `golangci-lint` requires a repository-owned `.golangci.yml`

---

## 14) Project command block

```bash
# validation
go test ./...
go vet ./...
golangci-lint run ./...
go build ./...

# formatting
go fmt ./...
```

