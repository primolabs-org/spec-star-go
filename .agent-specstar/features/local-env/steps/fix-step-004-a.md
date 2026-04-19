# Fix Step 004-A — Fail-fast on assertion failure

## Metadata
- Feature: local-env
- Fixes: step-004, CR-11
- Status: pending
- Last Updated: 2026-04-19

## Gap

`invoke_and_assert` in `e2e-test.sh` uses a `FAILURES` accumulator to defer exit instead of exiting immediately on statusCode mismatch. This violates the step's Failure Model ("hard failure with a non-zero exit") and the repository-wide fail-fast instructions.

## Required Change

1. In the `invoke_and_assert` function, replace the FAIL branch (`FAILURES=$((FAILURES + 1))`) with `exit 1`.
2. Remove the `FAILURES=0` variable declaration.
3. Remove the final `if [[ $FAILURES -gt 0 ]]` block and associated summary output.
4. Keep the `echo "All tests passed."` line at the end (it is reachable only if no assertion failed).

## Allowed Write Paths

- `e2e-test.sh`

## Scope

Four lines changed, three lines removed. No structural redesign.
