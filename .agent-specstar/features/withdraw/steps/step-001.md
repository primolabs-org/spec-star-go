# Step 001 - Add ErrInsufficientPosition Domain Error

## Metadata
- Feature: withdraw
- Step: step-001
- Status: pending
- Depends On: none
- Last Updated: 2026-04-19

## Objective

Add a sentinel error `ErrInsufficientPosition` to the domain errors package so the withdraw service and handler can classify and distinguish insufficient-position failures using `errors.Is`.

## In Scope

- New `ErrInsufficientPosition` sentinel in `internal/domain/errors.go`, following the existing `ErrConcurrencyConflict` and `ErrDuplicate` pattern.
- Test in `internal/domain/errors_test.go` verifying `errors.Is` classification after wrapping, matching the style of existing sentinel error tests.

## Out of Scope

- Application service code.
- Handler code.
- Any other domain changes.

## Required Reads

- `internal/domain/errors.go` — existing sentinel error definitions and `ValidationError` type.
- `internal/domain/errors_test.go` — existing test patterns for sentinel errors.

## Allowed Write Paths

- `internal/domain/errors.go`
- `internal/domain/errors_test.go`

## Forbidden Paths

- `internal/application/`
- `internal/adapters/`
- `internal/ports/`
- `cmd/`

## Known Abstraction Opportunities

None.

## Allowed Abstraction Scope

None.

## Required Tests

- `TestErrInsufficientPosition_ClassifiableAfterWrapping`: wrap `ErrInsufficientPosition` with `fmt.Errorf("context: %w", domain.ErrInsufficientPosition)` and assert `errors.Is` returns true.

## Coverage Requirement

100% on changed lines.

## Failure Model

Compile-time only. If the sentinel is malformed, tests fail.

## Allowed Fallbacks

None.

## Acceptance Criteria

1. `domain.ErrInsufficientPosition` is a package-level `var` of type `error`, defined via `errors.New("insufficient position")`.
2. `errors.Is(fmt.Errorf("context: %w", domain.ErrInsufficientPosition), domain.ErrInsufficientPosition)` returns `true`.
3. Test follows the naming and assertion style of existing sentinel error tests.
4. All existing tests pass.

## Deferred Work

None.

## Escalation Conditions

None expected.
