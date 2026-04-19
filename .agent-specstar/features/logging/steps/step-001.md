# Step 001 — Platform: LoggerFactory and context propagation helpers

## Objective

Create the `LoggerFactory` type and context propagation helpers in `internal/platform/` that all downstream consumers depend on.

## In Scope

- `LoggerFactory` struct with `NewLoggerFactory(service string, level slog.Leveler) *LoggerFactory`
- `FromContext(ctx context.Context, trigger string, operation string) *slog.Logger` method
- Cold start tracking (first call returns `cold_start=true`, subsequent calls return `false`)
- Lambda request ID extraction via `lambdacontext.FromContext(ctx)`. When Lambda context is absent, `request_id` is omitted from the logger attributes.
- `NewLoggerFactory` sets `slog.SetDefault()` with a JSON handler writing to `os.Stdout` and a base `service` attribute.
- `WithLogger(ctx, logger)` and `LoggerFromContext(ctx)` context helpers using an unexported `loggerKey{}` context key.
- `LoggerFromContext` returns `slog.Default()` when no logger is in context.
- Full unit test coverage.

## Out of Scope

- Modifying any consumer (handlers, services, repos, bootstrap).
- Adding logging calls anywhere outside `internal/platform/`.
- Any output format other than JSON.
- Log level configuration from environment variables (hardcoded `slog.Leveler` parameter is sufficient).
- `slog.Handler` abstraction or pluggable handler pattern.

## Required Reads

- `internal/platform/database.go` — understand existing platform package structure.
- `.github/skills/go-lambda-structured-logging/SKILL.md` — field model and bootstrap rules.
- `.agent-specstar/features/logging/design.md` — Technical Architecture section for exact API contract and field model.

## Allowed Write Paths

- `internal/platform/logger.go` (CREATE)
- `internal/platform/logger_test.go` (CREATE)

## Forbidden Paths

- `internal/platform/database.go`
- `internal/platform/database_test.go`
- Any file outside `internal/platform/`

## Known Abstraction Opportunities

- None. `LoggerFactory` is the minimal stable abstraction for this feature.

## Allowed Abstraction Scope

- None beyond what is specified.

## Required Tests

All in `internal/platform/logger_test.go`:

1. `NewLoggerFactory` produces a factory whose `FromContext` returns a logger that emits valid JSON with `service` field.
2. `FromContext` with Lambda context includes `request_id`, `trigger`, `operation`, `cold_start` fields.
3. `FromContext` without Lambda context omits `request_id` but includes other fields.
4. Cold start tracking: first `FromContext` call returns `cold_start=true`, second returns `cold_start=false`.
5. `WithLogger` / `LoggerFromContext` round-trip: stored logger is retrieved.
6. `LoggerFromContext` returns `slog.Default()` when context has no logger.
7. `NewLoggerFactory` sets `slog.SetDefault` — verify that `slog.Default()` output is JSON with `service` field.
8. Test log output verification uses `bytes.Buffer`-backed `slog.JSONHandler` (inject via test-only constructor or capture stdout).

## Coverage Requirement

100% on all lines in `internal/platform/logger.go`.

## Failure Model

- `NewLoggerFactory` does not fail. It creates in-memory objects only.
- `FromContext` does not fail. Missing Lambda context is handled by omitting `request_id`.
- `LoggerFromContext` does not fail. Missing logger is handled by returning `slog.Default()`.

## Allowed Fallbacks

- `LoggerFromContext` returning `slog.Default()` when no logger is in context is an explicit design decision documented in `design.md`, not a failure recovery mechanism.

## Acceptance Criteria

1. `internal/platform/logger.go` compiles with no errors.
2. `go test ./internal/platform/...` passes with 100% coverage on `logger.go`.
3. No changes to any file outside `internal/platform/`.
4. All existing tests in the repository continue to pass.
5. JSON output from the factory-created logger contains `service`, `trigger`, `operation`, and `cold_start` fields.
6. `request_id` is present when Lambda context is available and absent when it is not.

## Deferred Work

- None. This step is self-contained.

## Escalation Conditions

- If `lambdacontext.FromContext` cannot be tested without the Lambda runtime, escalate to determine a test strategy (the `lambdacontext.NewContext` function should enable this — verify before escalating).
- If `slog.SetDefault` in `NewLoggerFactory` causes test pollution across packages, escalate to determine isolation strategy.
