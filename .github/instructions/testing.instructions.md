---
applyTo: "**"
---

# Testing Instructions

Follow these rules when modifying code in this repository.

## Default expectation

- Every change that affects executable behavior must be validated by tests.
- Every changed executable line must receive test coverage unless the active task explicitly allows an exception.
- Do not treat existing overall coverage as a substitute for missing coverage on newly changed behavior.

## Required behavior

- Add new tests when introducing new behavior.
- Update existing tests when changing existing behavior.
- Remove or update obsolete tests when they no longer match the implementation.
- Keep tests aligned with the actual contract and behavior of the changed code.
- Make test coverage part of the same delivery as the implementation.

## Scope and relevance

- Test the behavior changed by the active task.
- Prefer focused tests with clear intent over broad, indirect coverage.
- Keep tests close to the style and structure already used in the affected area.
- Do not add unrelated test cases outside the active scope unless they are required to keep the changed area correct.

## Quality rules

- Tests must validate real behavior, not just execution of code paths.
- Tests must fail for the defect or regression they are meant to catch.
- Prefer deterministic tests over timing-sensitive, flaky, or environment-dependent tests.
- Keep assertions specific enough to detect incorrect behavior clearly.
- Do not use weak assertions that allow broken logic to pass unnoticed.
- Test coverage must be **above** 90% on the codebase level, and 100% on the changed lines.

## Regression discipline

- When fixing a bug, add or update a test that would fail without the fix.
- When refactoring behavior-preserving code, keep existing coverage valid or update it to reflect structural changes.
- Do not leave broken or outdated tests behind after modifying the implementation.

## Forbidden behavior

- Do not skip test work because the change looks small.
- Do not rely on manual reasoning as a substitute for automated validation.
- Do not leave changed behavior without coverage unless the active task explicitly allows it.
- Do not add placeholder tests, empty test cases, or assertions with no real verification value.
- Do not silence failing tests without resolving the underlying cause or documenting an explicitly approved deferral.

## Deferred testing policy

Only defer test work when the active task or step explicitly allows it.

When test work is explicitly deferred:
- keep the deferral narrow
- state what remains uncovered
- state why it is deferred
- state which future step is responsible for resolving it

## Test maintenance

- Remove dead, duplicated, or misleading tests exposed by the change in touched areas.
- Keep test names, fixtures, and setup aligned with the current implementation.
- Do not preserve obsolete test structure or comments after refactors.

## Examples of acceptable behavior

- Adding a regression test for a bug fix.
- Updating an existing test when a response contract changes.
- Adding focused tests for a new branch, validation rule, or error path.
- Removing or rewriting tests that became incorrect because of the active change.

## Examples of unacceptable behavior

- Changing behavior without adding or updating tests.
- Claiming coverage is sufficient because repository-level coverage is already high.
- Adding tests that execute code but do not verify meaningful outcomes.
- Leaving outdated tests that validate the old behavior after a refactor.
- Deferring test work without explicit approval in the active task or step.