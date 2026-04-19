---
applyTo: "**"
---

# Error Handling Instructions

Follow these rules when implementing, modifying, or reviewing code in this repository.

## Default behavior

- Handle errors explicitly and predictably.
- Prefer clear failure behavior over hidden recovery.
- Treat error handling as part of the contract of the code, not as an afterthought.
- Keep error paths as intentional and understandable as happy paths.

## Required behavior

- Validate required inputs, dependencies, and assumptions before proceeding with work that depends on them.
- Surface failures at the boundary where they become meaningful to the caller or operator.
- Preserve the original cause and relevant context when propagating or transforming an error.
- Translate errors only when crossing a clear boundary such as domain, transport, process, or external interface.
- Ensure error behavior matches the responsibility of the layer where it occurs.
- Keep error handling logic simple enough that a reviewer can reason about it without guessing.

## Error boundary discipline

- Handle errors at the level that can make a real decision about them.
- Do not catch errors in a lower layer if that layer cannot recover meaningfully.
- Do not leak internal implementation details through outward-facing error contracts.
- Convert internal failures into boundary-appropriate errors only when required by the active interface or design.
- Keep domain errors, validation errors, infrastructure failures, and unexpected failures distinguishable.

## Unreachable error discipline

- Propagate errors even when current preconditions make them provably unreachable.
- Preconditions change; error handling must remain correct independent of caller assumptions.
- Do not use `_ =` to discard error returns. Every error return must be checked.
- A comment explaining why an error "cannot happen" does not justify discarding it.

## Forbidden behavior

- Do not swallow exceptions, errors, or failed results.
- Do not catch an error only to ignore it.
- Do not add fallback behavior unless it is explicitly required.
- Do not invent success values when execution has failed.
- Do not return partial or misleading results that hide a failed operation.
- Do not duplicate error handling in multiple layers when one clear owner is sufficient.
- Do not hide the root cause by replacing an error with a vague generic message without preserving context.
- Do not treat logging alone as error handling.
- **Don't ever discard errors at any circumstance.** Even though it might seem like its justfiable at the current moment, it will leave traps after refactors and future changes.

## Recovery policy

Only recover from an error when recovery is explicitly required and the recovery path is well-defined.

When recovery is explicitly required:
- keep it narrow
- make the trigger explicit
- make the recovery outcome explicit
- preserve enough context to understand the original failure
- ensure the recovery path does not hide a real defect

## Scope discipline

- Keep error handling aligned with the active task and the existing contract.
- Do not introduce broad defensive error branches for hypothetical future cases.
- Do not widen an interface contract just to avoid dealing with a real failure.
- Do not add abstractions around errors unless they make behavior clearer.

## Message discipline

- Error messages must be direct, specific, and actionable where appropriate.
- Do not expose secrets, credentials, or sensitive internal details in error messages.
- Do not use vague messages that make distinct failure modes indistinguishable.
- Keep externally surfaced messages safe and internally diagnosable.

## Examples of acceptable behavior

- Rejecting invalid input with an explicit error.
- Propagating a failure with preserved cause and added local context.
- Translating an infrastructure failure into a boundary-appropriate application error.
- Stopping execution when a required dependency or invariant is missing.

## Examples of unacceptable behavior

- Catching an error and returning an empty result to keep execution moving.
- Replacing a specific failure with a generic message that loses the cause.
- Logging an error and then continuing as if the operation succeeded.
- Adding broad retry or recovery logic without an explicit requirement.
- Mixing multiple unrelated failure modes into one indistinguishable output.