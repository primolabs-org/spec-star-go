---
applyTo: "**"
---

# Logging Instructions

Follow these rules when implementing, modifying, or reviewing code in this repository.

## Default behavior

- Use logging to make important execution behavior diagnosable.
- Prefer structured, contextual, low-noise logging over ad hoc message dumping.
- Treat logs as an operational diagnostic tool, not as a substitute for code correctness.
- Log events that help explain state transitions, decisions, failures, and outcomes.

## Required behavior

- Emit logs that make it possible to understand what the system attempted, what happened, and why it failed when failure occurs.
- Include relevant execution context in logs when that context exists.
- Keep log messages specific enough to distinguish meaningful events.
- Choose log levels intentionally based on operational significance.
- Ensure important failures, retries, and terminal outcomes are logged at the appropriate level.
- Keep logs aligned with real execution boundaries such as requests, jobs, tasks, commands, consumers, or workflows.

## Context discipline

- Include identifiers and contextual fields that help correlate related events when available.
- Keep the same conceptual fields consistent across similar execution paths.
- Prefer machine-parseable fields over free-form text when context must be preserved.
- Make logs useful to both humans and downstream tooling.

## Forbidden behavior

- Do not log secrets, credentials, tokens, raw keys, or sensitive personal data.
- Do not log entire payloads or object dumps unless explicitly required and safe.
- Do not use logs as a substitute for returning values, raising errors, or enforcing contracts.
- Do not add noisy logs for trivial happy-path steps with no diagnostic value.
- Do not log the same failure repeatedly across multiple layers without a clear reason.
- Do not emit vague messages that make distinct failures look identical.
- Do not leave temporary debug logging behind after the active task is complete.

## Level discipline

- Use higher-severity logs for failures that require attention or indicate lost work, degraded service, or contract violations.
- Use lower-severity logs for diagnostic detail that is helpful but not operationally urgent.
- Do not overstate routine behavior as an error.
- Do not understate terminal failures as harmless informational noise.

## Failure logging discipline

- Log failures at the boundary where they become operationally meaningful.
- Include enough context to diagnose the failure without duplicating the full stack of lower-level logs.
- When an error is propagated upward, avoid re-logging it at every layer unless each layer adds distinct value.
- Keep one clear owner for terminal failure logging whenever possible.

## Scope discipline

- Add or update logging only where it improves observability of the changed behavior.
- Do not turn a narrow change into a repository-wide logging rewrite.
- Do not introduce logging abstractions unless they improve clarity or consistency.

## Message discipline

- Keep log messages short, specific, and stable in meaning.
- Prefer messages that describe what happened over decorative prose.
- Ensure a reviewer can tell why the log exists and what action or diagnosis it supports.
- Keep dynamic values in structured fields when possible instead of embedding everything into free text.

## Examples of acceptable behavior

- Logging the start and outcome of a background job with a job identifier.
- Logging a terminal failure with relevant context and clear severity.
- Logging a retry attempt with attempt count and reason.
- Removing temporary debug logs once the implementation is complete.

## Examples of unacceptable behavior

- Dumping full request or response bodies without a clear requirement.
- Logging the same exception in every layer it passes through.
- Leaving verbose debug statements in production paths after the task is done.
- Using vague messages like "something went wrong" without distinguishing context.
- Logging sensitive values that should never leave process memory.