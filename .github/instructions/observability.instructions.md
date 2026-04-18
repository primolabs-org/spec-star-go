---
applyTo: "**"
---

# Observability Instructions

Follow these rules when implementing, modifying, or reviewing code in this repository.

## Default behavior

- Build systems so important runtime behavior can be understood from the outside.
- Treat observability as part of delivery quality, not as optional polish.
- Ensure changed behavior is diagnosable in production-like operation without reading code first.
- Prefer intentional instrumentation of important paths over broad, noisy instrumentation with unclear value.

## Required behavior

- Instrument critical execution paths so operators and developers can determine throughput, latency, failures, and outcomes.
- Ensure failures, retries, degraded behavior, and terminal states are observable.
- Correlate logs, metrics, and traces when the execution model supports it.
- Make it possible to follow a meaningful unit of work across system boundaries when the active design requires it.
- Add or update instrumentation when the active task changes runtime behavior in a way that affects diagnosis or operations.
- Keep observability aligned with real business and operational boundaries such as requests, jobs, workflows, commands, or message handling.

## Signal discipline

- Use logs for event detail and diagnostic context.
- Use metrics for rates, counts, durations, saturation, and outcome trends.
- Use traces or equivalent execution correlation for following work across boundaries when supported.
- Choose the signal type that best matches the question an operator will need to answer.
- Do not force every concern into a single signal type.

## Critical path discipline

- Instrument paths where failures, latency, retries, handoffs, or data loss would matter.
- Ensure important dependencies and external calls are observable when they affect outcome or timing.
- Capture enough context to distinguish success, failure, retry, timeout, cancellation, and partial completion where those states exist.
- Keep instrumentation focused on points that support real diagnosis and operations.

## Forbidden behavior

- Do not add instrumentation with no clear operational or diagnostic purpose.
- Do not emit high-cardinality or unbounded dimensions without explicit approval.
- Do not create telemetry that cannot be tied back to a meaningful unit of work.
- Do not treat logs alone as a complete observability strategy when metrics or traces are needed.
- Do not collect sensitive values in telemetry.
- Do not duplicate the same signal in multiple places without a clear reason.
- Do not leave changed runtime behavior without corresponding observability when diagnosis would otherwise become harder.

## Correlation discipline

- Preserve correlation context across boundaries when the platform or design supports it.
- Keep identifiers stable enough to connect related telemetry for a single unit of work.
- Ensure different telemetry types can be joined conceptually around the same execution.
- Do not break propagation or correlation in changed code without an explicit reason.

## Alerting and operational usefulness

- Instrumentation should support operational questions such as:
  - Is it working?
  - Is it failing?
  - Is it slow?
  - Is it retrying?
  - Is work being lost, duplicated, or stuck?
- Prefer instrumentation that helps detect and localize faults over instrumentation that only confirms happy-path execution.
- Ensure emitted signals are meaningful enough to support dashboards, alerting, or incident diagnosis where applicable.

## Scope discipline

- Add or adjust observability within the scope of the changed behavior.
- Do not turn a narrow task into a full observability redesign unless explicitly required.
- Do not introduce large telemetry frameworks or abstractions without a real need in the active task.

## Examples of acceptable behavior

- Emitting duration and outcome metrics for a critical operation.
- Preserving correlation context across a workflow boundary.
- Instrumenting retries, failures, and terminal outcomes for a background process.
- Adding telemetry to a newly introduced dependency call that affects latency or correctness.

## Examples of unacceptable behavior

- Adding generic logs but no way to measure error rate or latency for a critical path.
- Emitting telemetry with identifiers so unique that aggregation becomes useless.
- Breaking correlation between related operations after a refactor.
- Instrumenting every minor step while leaving critical boundaries invisible.
- Capturing sensitive payload data in traces, metrics, or logs.