# Logging — Introduce Structured Operational Logging

## Problem Statement

The spec-star-go service has **no operational logging** beyond two `log.Fatalf` calls at startup. HTTP handlers, application services, and repository adapters produce no log output. This makes production troubleshooting, incident diagnosis, and operational monitoring effectively impossible.

The user reported this as a "`fmt` to `log` migration" problem, but analysis reveals that all `fmt` usage in production code is `fmt.Errorf` (standard Go error wrapping) and `fmt.Sprintf` (string formatting) — neither of which is logging. The actual gap is the complete absence of operational log emission at any layer.

## Goal

Add structured operational logging across the service so that operators can answer:

- Is the service processing requests?
- What requests are failing, and why?
- What downstream dependencies are failing?
- Can related log entries for a single request be correlated?

## Functional Requirements

### FR-1: Centralized logger bootstrap

A single logger must be created during Lambda bootstrap in `cmd/http-lambda/main.go` and reused across warm invocations. Individual packages must not create their own loggers independently.

### FR-2: Request correlation

Every log entry produced during a request must carry a `request_id` field that allows correlating all log entries for a single Lambda invocation.

### FR-3: Inbound adapter logging

HTTP handlers must log:

- Terminal error outcomes (4xx classification failures, 5xx internal errors) with route, method, status code, and error context.
- Request start is optional and only required if explicitly decided during implementation.

### FR-4: Application service logging

Application services must log:

- Terminal operation failures with operation name and relevant entity identifiers.
- Idempotency replay events (when a processed command is replayed) at an appropriate level.

### FR-5: Outbound adapter logging

Repository adapters must log:

- Dependency failures (database query errors, connection errors) with operation context.
- Must NOT log full query parameters or sensitive data.

### FR-6: Startup logging

The existing `log.Fatalf` calls in `main.go` must be migrated to use the new structured logger for consistency.

### FR-7: Stable field model

Log entries must use a consistent field model including at minimum:

- `service` — service name
- `trigger` — `http`
- `operation` — the logical operation being performed
- `request_id` — invocation correlation identifier
- `level` — log severity
- `error` — error detail when applicable

## Non-Functional Requirements

### NFR-1: Structured JSON output

Log output must be JSON-structured for machine parsing in AWS CloudWatch and compatible with Lambda advanced logging controls.

### NFR-2: Standard library only

Use Go's `log/slog` package (standard library since Go 1.21). No third-party logging libraries.

### NFR-3: Low noise

Logs must be operationally meaningful. Do not log every happy-path step. Focus on failures, terminal outcomes, decisions, and correlation-relevant lifecycle events.

### NFR-4: No secrets in logs

Logs must never contain credentials, tokens, full request/response bodies, or sensitive business identifiers.

### NFR-5: Performance

Logger construction must happen once at bootstrap. Per-request enrichment must use `logger.With(...)` or equivalent lightweight patterns.

## Scope

### In scope

| Layer | Directory | What changes |
|---|---|---|
| Bootstrap | `cmd/http-lambda/` | Logger creation, injection, startup log migration |
| Platform | `internal/platform/` | Logger factory or bootstrap helper |
| HTTP handlers | `internal/adapters/inbound/httphandler/` | Request correlation, terminal error logging |
| Application services | `internal/application/` | Operation outcome logging, idempotency replay logging |
| Outbound adapters | `internal/adapters/outbound/postgres/` | Dependency failure logging |

### Out of scope

- Replacing `fmt.Errorf` calls — these are standard Go error wrapping, not logging, and must be preserved.
- Replacing `fmt.Sprintf` calls — these are string formatting, not logging.
- Metrics or tracing instrumentation.
- Log-based alerting rules or CloudWatch alarm configuration.
- SQS trigger logging (no SQS trigger exists in this service).
- Domain layer logging (`internal/domain/`) — domain code returns errors to callers per hexagonal architecture.

## Constraints and Assumptions

1. **Go version ≥ 1.21** is required for `log/slog`. The go.mod must already satisfy this or must be updated.
2. **Hexagonal architecture** is preserved. The logger is injected through constructors or context, not imported as a global.
3. **`fmt.Errorf` is preserved as-is.** It is idiomatic Go error wrapping and is not a logging concern.
4. The existing `go-lambda-structured-logging` skill in `.github/skills/` provides implementation patterns that should be followed.
5. AWS Lambda request ID is available via the Lambda context and should be used as `request_id`.

## Existing Context

- **Architecture:** Hexagonal — `cmd/` → `adapters/inbound/httphandler/` → `application/` → `ports/` ← `adapters/outbound/postgres/`
- **Trigger:** AWS Lambda behind API Gateway HTTP API v2.
- **Database:** PostgreSQL via `pgx`.
- **Current logging:** Only `log.Fatalf` in `cmd/http-lambda/main.go` (2 calls at startup).
- **Existing skill:** `.github/skills/go-lambda-structured-logging/SKILL.md` defines the full `log/slog` pattern including `LoggerFactory`, JSON handler, field model, and level policy.
- **Ports pattern:** Services accept interfaces via constructors. The logger can follow the same injection pattern.

## Technical Architecture

### Logger injection model

- **`LoggerFactory`** is injected via constructor into HTTP handlers only. Handlers are the entry point that creates per-request correlated loggers.
- **Per-request correlated logger** is stored in `context.Context` by the handler after calling `factory.FromContext(ctx, trigger, operation)`.
- **Application services and outbound adapters** retrieve the correlated logger from context via `platform.LoggerFromContext(ctx)`. No constructor changes to services or repositories.
- **`LoggerFromContext`** returns `slog.Default()` when no logger is in context. This is an explicit design decision for test ergonomics: tests that do not care about logging run without setup. In production, bootstrap configures `slog.SetDefault` with the JSON handler, so even the fallback produces structured output.

### LoggerFactory API contract

```go
// internal/platform/logger.go

type LoggerFactory struct { /* base logger, service name, cold start state */ }

func NewLoggerFactory(service string, level slog.Leveler) *LoggerFactory
func (f *LoggerFactory) FromContext(ctx context.Context, trigger string, operation string) *slog.Logger
```

- `NewLoggerFactory` creates a `slog.JSONHandler` writing to `os.Stdout`, constructs a base `*slog.Logger` with `service` attribute, and calls `slog.SetDefault()` so startup logging uses the same format.
- `FromContext` returns a `*slog.Logger` enriched with `trigger`, `operation`, `request_id` (extracted from Lambda context via `lambdacontext.FromContext`), and `cold_start`.
- **Cold start tracking:** the first call to `FromContext` returns `cold_start=true`. All subsequent calls return `cold_start=false`. This is safe in Lambda because invocations are serial per instance.

### Context propagation helpers

```go
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context
func LoggerFromContext(ctx context.Context) *slog.Logger
```

- `WithLogger` stores a `*slog.Logger` in context under an unexported key.
- `LoggerFromContext` retrieves the logger. Returns `slog.Default()` if absent.
- This pattern mirrors the existing `executorFromContext` / `txKey{}` pattern in `internal/adapters/outbound/postgres/helpers.go`.

### Consumer-defined interface for handlers

HTTP handlers define a consumer-side interface rather than importing the concrete `*LoggerFactory` type:

```go
type loggerFactory interface {
    FromContext(ctx context.Context, trigger, operation string) *slog.Logger
}
```

This keeps handler tests simple: tests inject a mock or noop factory. The concrete `*platform.LoggerFactory` satisfies this interface implicitly.

Handlers import `platform` only for the `WithLogger` standalone function (to store the enriched logger in context before calling the service).

### Field model

| Field | Source | Type | Present at |
|---|---|---|---|
| `service` | `LoggerFactory` base | `string` | All entries |
| `trigger` | `FromContext` parameter | `string` | All request entries |
| `operation` | `FromContext` parameter | `string` | All request entries |
| `request_id` | Lambda context | `string` | All request entries |
| `cold_start` | `LoggerFactory` state | `bool` | All request entries |
| `error` | Per-log-call | `string` | Error/warn entries |
| `outcome` | Per-log-call | `string` | Terminal outcome entries |
| `status` | Per-log-call | `int` | Handler entries |
| `client_id` | Per-log-call | `string` | Service entries with entity context |
| `order_id` | Per-log-call | `string` | Service entries with idempotency context |

### Log emission points

**HTTP handlers** — log when `service.Execute` returns an error:
- `status >= 500` → `logger.Error(...)` with `status`, `error`, `outcome=failed`
- `status < 500` → `logger.Warn(...)` with `status`, `error`, `outcome=failed`
- Body parse failures and method-not-allowed are NOT logged (trivial HTTP boundary issues with no diagnostic value).

**Application services** — log terminal failures and idempotency replays:
- `5xx` error paths (infrastructure failures) → `logger.Error(...)` with `error`, `outcome=failed`, entity identifiers
- Idempotency replays (existing processed command found) → `logger.Info(...)` with `order_id`, `outcome=replayed`
- Race condition replays (`ErrDuplicate` → re-read) → `logger.Info(...)` with `order_id`, `outcome=replayed`
- `4xx` validation failures (client not found, asset not found) are NOT logged at service level — they are expected business outcomes, not operational signals.

**Outbound adapters** — log infrastructure-level database failures:
- Unexpected query/exec errors (NOT `pgx.ErrNoRows`, NOT `domain.ErrDuplicate`) → `logger.Error(...)` with entity identifier and error detail.
- Transaction `Begin` and `Commit` failures → `logger.Error(...)`.
- `ErrNoRows` mapped to `domain.ErrNotFound` is a business outcome — NOT logged.
- `pgUniqueViolation` mapped to `domain.ErrDuplicate` is a business outcome — NOT logged.

### Level policy

| Level | Usage |
|---|---|
| `ERROR` | Terminal infrastructure failures, dependency failures, 5xx outcomes |
| `WARN` | Client-caused failures (4xx) that are terminal for the request |
| `INFO` | Idempotency replays, meaningful lifecycle events |
| `DEBUG` | Not used initially |

### Bootstrap sequence

1. Create `LoggerFactory` (sets `slog.SetDefault` with JSON handler + `service` field)
2. Load database config — on failure: `slog.Error(...)` + `os.Exit(1)` (replaces `log.Fatalf`)
3. Create database pool — on failure: same pattern
4. Create outbound adapters (repos, transaction runner) — unchanged constructors
5. Create application services — unchanged constructors
6. Create HTTP handlers — inject `LoggerFactory` into constructors
7. Start Lambda handler

### Testing approach

- **LoggerFactory tests:** use `slog.NewJSONHandler` backed by `bytes.Buffer` to capture output. Verify field presence (`service`, `trigger`, `operation`, `request_id`, `cold_start`), cold start flip behavior, and missing Lambda context behavior.
- **Handler tests:** define a `mockLoggerFactory` that returns a test logger backed by a buffer. Verify that terminal errors produce log entries with expected fields and levels.
- **Service tests:** set up test logger via `platform.WithLogger(ctx, testLogger)` before calling `Execute`. Verify log entries for terminal failures and idempotency replays.
- **Outbound adapter tests:** create unit tests with mock `DBTX` injected via `txKey{}` context pattern (same as existing transaction context pattern). Trigger infrastructure errors and verify log output.

## Success Criteria

1. Every Lambda invocation that results in an error produces at least one structured log entry with `request_id`, `operation`, and `error` fields.
2. All log entries for a single invocation can be correlated by `request_id` in CloudWatch.
3. Log output is valid JSON parseable by CloudWatch Logs Insights.
4. No `fmt.Errorf` or `fmt.Sprintf` calls are removed or broken.
5. No secrets, tokens, or full request bodies appear in log output.
6. Logger is constructed once at bootstrap, not per-invocation.
7. Test coverage on changed lines meets repository standard (100% on changed lines, >90% codebase).

## Clarification Resolution

All clarifications were resolved before implementation. Recorded here for traceability.

| # | Question | Resolution |
|---|---|---|
| Q1 | Preserve `fmt.Errorf` as-is? | **Yes.** `fmt.Errorf` is idiomatic Go error wrapping, not logging. All `fmt` usage (`fmt.Errorf`, `fmt.Sprintf`) is preserved unchanged. |
| Q2 | Use `log/slog` as the logging standard? | **Yes.** `log/slog` is mandated by repository doctrine (skills and instructions). The basic `log` package is not used. |
| Q3 | What log level granularity? | **Standard model.** ERROR for terminal failures, INFO for meaningful lifecycle events (e.g., idempotency replays). WARN and DEBUG available but not required initially. |
| Q4 | JSON-structured output? | **Yes.** Required for CloudWatch and Docker log visibility. The absence of structured logging was hiding errors that did not surface in docker logs. |

## Technical Debt

Discovered during the in-scope audit. These are real issues in touched files but are not required for safe execution of the logging feature. Documented here for future resolution.

| # | File | Issue | Rationale for deferral |
|---|---|---|---|
| TD-1 | `internal/adapters/outbound/postgres/testhelper_test.go` | `_ = tx.Rollback(context.Background())` discards the rollback error in test cleanup. Violates error-handling instructions. | Test cleanup code only. Not related to logging feature. Fixing requires deciding on test cleanup error handling strategy. |
