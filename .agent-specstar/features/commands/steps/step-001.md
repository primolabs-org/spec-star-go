# Step 001 — Add aws-lambda-go dependency

## Objective

Add `github.com/aws/aws-lambda-go` to `go.mod` so subsequent steps can import Lambda event types and the runtime entrypoint.

## In Scope

- Running `go get github.com/aws/aws-lambda-go` to add the module.
- Verifying `go.mod` lists the dependency.

## Out of Scope

- Any Go source code changes.
- Any test changes.

## Required Reads

- `go.mod` — verify `aws-lambda-go` is not already present.

## Allowed Write Paths

- `go.mod`
- `go.sum`

## Forbidden Paths

- Any file outside `go.mod` and `go.sum`.

## Known Abstraction Opportunities

None.

## Allowed Abstraction Scope

None.

## Required Tests

None — no executable behavior changed.

## Coverage Requirement

N/A — dependency addition only.

## Failure Model

- If `go get` fails (network, version resolution), the step fails. No fallback.

## Allowed Fallbacks

None.

## Acceptance Criteria

1. `go.mod` contains `github.com/aws/aws-lambda-go` in the `require` block.
2. `go build ./...` succeeds.

## Deferred Work

None.

## Escalation Conditions

- If `aws-lambda-go` has an incompatible minimum Go version with `go 1.25.0`, escalate before proceeding.
