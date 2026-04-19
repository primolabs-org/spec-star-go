# Step 004 — Handler instrumentation: root spans and command metrics

## Metadata
- Feature: tracing
- Step: step-004
- Status: pending
- Depends On: step-001
- Last Updated: 2026-04-19

## Objective

Instrument `DepositHandler.Handle` and `WithdrawHandler.Handle` with root spans and command-level metrics. Each handler creates a span wrapping the full request lifecycle and records `wallet.command.count` and `wallet.command.duration` metrics.

## In Scope

- Modify `deposit_handler.go`:
  - Start a span named `POST /deposits` using `otel.Tracer("httphandler")`.
  - Span wraps the full `Handle` method (defer `span.End()`).
  - On success: set span status OK, set `wallet.outcome` attribute to `success`.
  - On failure: set span status Error with description, record the error, set `wallet.outcome` to `failed`.
  - On idempotent replay (detectable from status code 200 when service returns replayed response): set `wallet.outcome` to `replayed`.
  - Set attributes: `http.method`, `http.route` (`/deposits`), `wallet.command` (`deposit`).
  - Record `wallet.command.count` counter (attributes: `command=deposit`, `outcome`).
  - Record `wallet.command.duration` histogram (attributes: `command=deposit`, `outcome`).
- Modify `withdraw_handler.go`:
  - Same pattern as deposit handler with span name `POST /withdrawals`, `wallet.command` = `withdraw`, `http.route` = `/withdrawals`.
- Create or reuse a shared metrics helper in `httphandler` package if beneficial (e.g., package-level meter and instrument initialization).
- Update existing handler tests to verify:
  - Spans are created with expected names and attributes.
  - Metrics are recorded.
  - Use a test `TracerProvider` with `tracetest.SpanRecorder` or in-memory span exporter.
  - Use a test `MeterProvider` with SDK test reader if feasible, or verify at minimum that the code path executes without error.

## Out of Scope

- Application service child spans (step-005).
- Repository child spans (step-006).
- Trace-log correlation (step-002).
- Docker Compose changes (step-003).

## Required Reads

- `.agent-specstar/features/tracing/design.md` — FR-2, FR-5.
- `internal/adapters/inbound/httphandler/deposit_handler.go` — current implementation.
- `internal/adapters/inbound/httphandler/withdraw_handler.go` — current implementation.
- `internal/adapters/inbound/httphandler/deposit_handler_test.go` — existing test patterns.
- `internal/adapters/inbound/httphandler/withdraw_handler_test.go` — existing test patterns.
- `.github/skills/go-lambda-observability-otel/SKILL.md` — inbound adapter rules.

## Allowed Write Paths

- `internal/adapters/inbound/httphandler/deposit_handler.go` (MODIFY)
- `internal/adapters/inbound/httphandler/deposit_handler_test.go` (MODIFY)
- `internal/adapters/inbound/httphandler/withdraw_handler.go` (MODIFY)
- `internal/adapters/inbound/httphandler/withdraw_handler_test.go` (MODIFY)

## Forbidden Paths

- `internal/platform/**`
- `internal/application/**`
- `internal/adapters/outbound/**`
- `internal/domain/**`
- `cmd/**`

## Known Abstraction Opportunities

- A package-level `var tracer = otel.Tracer("httphandler")` and shared meter/instrument variables to avoid repeated initialization.
- A small helper for recording command metrics to reduce duplication between deposit and withdraw handlers.

## Allowed Abstraction Scope

- Package-level tracer variable and metric instruments in the `httphandler` package.
- One small unexported helper function for metrics recording, if it reduces duplication.

## Required Tests

In `deposit_handler_test.go`:
1. Successful deposit creates a span named `POST /deposits` with status OK and `wallet.outcome=success`.
2. Failed deposit (validation error) creates a span with status Error and `wallet.outcome=failed`.
3. Failed deposit (service error) creates a span with status Error and `wallet.outcome=failed`.
4. Span attributes include `http.method=POST`, `http.route=/deposits`, `wallet.command=deposit`.
5. `wallet.command.count` is incremented (or at minimum, the recording code path executes without error).

In `withdraw_handler_test.go`:
1. Successful withdrawal creates a span named `POST /withdrawals` with status OK and `wallet.outcome=success`.
2. Failed withdrawal creates a span with status Error and `wallet.outcome=failed`.
3. Span attributes include `wallet.command=withdraw`.

Test approach: Configure a test `TracerProvider` using `go.opentelemetry.io/otel/sdk/trace/tracetest` or equivalent. Capture exported spans and assert on names, attributes, and status.

## Coverage Requirement

100% on all changed lines in `deposit_handler.go` and `withdraw_handler.go`.

## Failure Model

- Span creation never fails (OpenTelemetry API returns noop spans if provider is misconfigured).
- Metric recording never fails (counters and histograms silently drop if provider is misconfigured).
- The handler must continue to return correct HTTP responses regardless of telemetry state.

## Allowed Fallbacks

- none

## Acceptance Criteria

1. Both handler files compile with no errors.
2. `go test ./internal/adapters/inbound/httphandler/...` passes with 100% coverage on changed lines.
3. Span names follow the convention: `POST /deposits`, `POST /withdrawals`.
4. Span attributes include `http.method`, `http.route`, `wallet.command`, `wallet.outcome`.
5. `wallet.command.count` and `wallet.command.duration` metrics are recorded with correct attributes.
6. Existing handler behavior (response codes, bodies, error handling) is unchanged.

## Deferred Work

- Idempotent replay detection may require the service to communicate replay vs fresh success. If the current service API (returning status code 200 for both replayed and fresh) does not distinguish them, defer `replayed` outcome tracking to a follow-up or resolve during implementation by inspecting how the handler can detect replays.

## Escalation Conditions

- If the handler cannot reliably distinguish `replayed` from `success` outcomes without changing the service interface, escalate the design question.
