# Step 002 — Platform: Trace-log correlation in LoggerFactory

## Metadata
- Feature: tracing
- Step: step-002
- Status: pending
- Depends On: step-001
- Last Updated: 2026-04-19

## Objective

Enhance the existing `LoggerFactory` in `internal/platform/logger.go` so that when an active OpenTelemetry span exists in the context, `trace_id` and `span_id` attributes are automatically injected into every log record. The primary logging path (`slog` → JSON → stdout) must remain unchanged.

## In Scope

- Modify the `slog.Handler` construction in `newLoggerFactory` to wrap the JSON handler with a custom handler that extracts `trace.SpanFromContext(ctx)` and injects `trace_id` and `span_id` attributes when the span is recording.
- When no active span exists or the span is not recording, no trace fields are added (no placeholders, no failure).
- The correlation handler must be transparent: all existing log fields (`service`, `trigger`, `operation`, `request_id`, `cold_start`) remain unchanged.
- Update existing tests in `logger_test.go` to cover trace correlation behavior.
- New test cases for: span present → trace_id and span_id appear; no span → fields absent.

## Out of Scope

- Routing logs through the OTel Logs SDK exporter.
- Any changes to `otel.go` or `main.go`.
- Handler, service, or repository instrumentation (steps 004–006).
- Docker Compose changes (step-003).

## Required Reads

- `.agent-specstar/features/tracing/design.md` — FR-6, Execution Notes (slog.Handler wrapper approach).
- `internal/platform/logger.go` — current implementation.
- `internal/platform/logger_test.go` — current test patterns.
- `.github/skills/go-lambda-structured-logging/SKILL.md` — field model rules.
- `.github/skills/go-lambda-observability-otel/SKILL.md` — correlation rules.

## Allowed Write Paths

- `internal/platform/logger.go` (MODIFY)
- `internal/platform/logger_test.go` (MODIFY)

## Forbidden Paths

- `internal/platform/otel.go`
- `internal/platform/otel_test.go`
- `internal/platform/database.go`
- `cmd/http-lambda/main.go`
- `internal/adapters/**`
- `internal/application/**`
- `internal/domain/**`

## Known Abstraction Opportunities

- A custom `slog.Handler` wrapper (e.g., `traceContextHandler`) that delegates to the underlying JSON handler and adds trace fields from span context. This is the preferred approach per the design document.

## Allowed Abstraction Scope

- One unexported `slog.Handler` wrapper type in `logger.go`. No new files.

## Required Tests

All in `internal/platform/logger_test.go`:

1. When a recording span is active in context, log output includes `trace_id` and `span_id` fields with non-empty hex values matching the span context.
2. When no span is in context, log output does not include `trace_id` or `span_id` fields.
3. When a non-recording (noop) span is in context, log output does not include `trace_id` or `span_id` fields.
4. All existing `LoggerFactory` tests continue to pass unchanged (cold start, request_id, trigger, operation, service fields).
5. The `slog.SetDefault` logger also inherits trace correlation behavior.

Test approach: Use `go.opentelemetry.io/otel/sdk/trace` test utilities to create a real span in a test `TracerProvider` with an in-memory exporter, then verify log output via a `bytes.Buffer`-backed handler.

## Coverage Requirement

100% on all changed lines in `internal/platform/logger.go`.

## Failure Model

- `trace.SpanFromContext(ctx)` never fails; it returns a noop span when no span exists.
- The correlation handler must not panic or fail. Missing trace context is a normal case, not an error.
- If `Handle` is called with a context that has a span, the trace fields are best-effort. If the span context is invalid (zero trace ID), no trace fields are added.

## Allowed Fallbacks

- Omitting trace fields when no valid span context exists is the intended behavior, not a fallback.

## Acceptance Criteria

1. `internal/platform/logger.go` compiles with no errors.
2. `go test ./internal/platform/...` passes with 100% coverage on changed lines in `logger.go`.
3. Log output with an active span includes `trace_id` and `span_id`.
4. Log output without a span does not include trace fields.
5. All existing logger tests pass without modification to their assertions (backward compatible).
6. The `logger.go` file gains a dependency on `go.opentelemetry.io/otel/trace` (API only, not SDK).

## Deferred Work

- none

## Escalation Conditions

- If the `slog.Handler` interface makes it difficult to inject attributes per-record without losing group context, document the limitation and propose an alternative approach.
