---
applyTo: "**"
---

# Fail-Fast Instructions

Follow these rules when implementing, modifying, or reviewing code in this repository.

## Default behavior

- Prefer fail-fast over fail-safe unless the active task explicitly requires fail-safe behavior.
- Do not hide invalid state, missing data, misconfiguration, or contract violations.
- Surface problems at the point they are detected instead of masking them and continuing silently.

## Forbidden behaviors

- Do not add silent fallbacks.
- Do not guess default values for unresolved or missing inputs.
- Do not auto-correct invalid data unless the task explicitly requires normalization.
- Do not swallow exceptions, errors, or failed validations.
- Do not return placeholder values that make broken behavior appear valid.
- Do not preserve broken legacy paths unless the task explicitly requires compatibility behavior.

## Required behaviors

- Validate required inputs, assumptions, and dependencies explicitly.
- Fail with clear and direct error behavior when required conditions are not met.
- Keep error handling explicit and easy to reason about.
- Prefer immediate rejection over hidden recovery when a condition would otherwise produce incorrect behavior.
- When the correct behavior is unclear, stop and surface the ambiguity instead of inventing behavior.

## Fallback policy

Only introduce fallback or recovery behavior when it is explicitly required by:
- the active task
- the design
- an approved project convention

When fallback behavior is explicitly required:
- keep it narrow
- make it visible in code
- do not let it hide real defects
- document the intended trigger and outcome in the implementation

## Scope discipline

- Do not add defensive branches “just in case” without a real requirement.
- Do not widen behavior to handle hypothetical future scenarios.
- Prefer strict, predictable behavior over permissive behavior that increases ambiguity.

## Examples of acceptable behavior

- Rejecting missing required inputs instead of inventing defaults.
- Raising or returning an explicit error when a dependency contract is violated.
- Stopping implementation work when the correct behavior depends on unresolved ambiguity.

## Examples of unacceptable behavior

- Falling back to an empty value to avoid a crash.
- Inventing default business values when the source value is missing.
- Catching an error and continuing as if nothing happened.
- Adding broad recovery logic that was not part of the requirement.
- Leaving fallbacks behind after removing/refactoring code assuming retrocompatibility behavior unless explicitly required in step files.