# Step 002 - Pin Go toolchain, builder image, and refresh direct dependencies

## Metadata
- Feature: local-env-fixes
- Step: step-002
- Status: pending
- Depends On: step-001
- Last Updated: 2026-04-19

## Objective

Align every repository-owned Go toolchain reference with **Go 1.26.2**, pin the Docker builder image to **`golang:1.26.2-bookworm`**, and refresh direct Go module dependencies to current stable versions. Satisfies FR-1, FR-2, FR-3, FR-15, FR-16, FR-17, FR-18, and FR-19 of `design.md`.

## In Scope

- `go.mod`: change `go 1.25.0` to `go 1.26.2`. Add an explicit `toolchain go1.26.2` directive only if the local Go binary is not exactly 1.26.2.
- `Dockerfile`: change the builder stage to `FROM golang:1.26.2-bookworm AS build`. The runtime stage stays on `public.ecr.aws/lambda/provided:al2023`.
- `go.mod` direct `require` block: upgrade each direct dependency to the latest stable version available at implementation time using `go get -u <pkg>@latest`. Run `go mod tidy` afterwards.
- If `cmd/http-lambda/main.go` directly imports `github.com/aws/aws-lambda-go`, ensure that package surfaces in the direct `require` block after `go mod tidy`.

## Out of Scope

- Editing `docker-compose.yml` (next step).
- Editing `.github/agents/AGENTS.md` (final verification step).
- Introducing new direct dependencies.
- Bumping the Lambda runtime base image to a non-`al2023` family.
- Reformatting or refactoring Go source beyond the smallest change required to keep `go test ./...` green after the dependency upgrade.
- Adjusting `.golangci.yml`.

## Required Reads

- `.agent-specstar/features/local-env-fixes/design.md` (sections "Toolchain", "Builder image", "Direct dependency refresh", "Failure Model")
- `go.mod`
- `Dockerfile`
- `cmd/http-lambda/main.go`
- `.github/agents/AGENTS.md` (section 4)
- `.github/instructions/cleanup.instructions.md`
- `.github/instructions/fail-fast.instructions.md`
- `.github/instructions/error-handling.instructions.md`
- `.github/instructions/testing.instructions.md`

## Allowed Write Paths

- `go.mod`
- `go.sum`
- `Dockerfile`
- Go source files under `cmd/` and `internal/` ONLY when a dependency upgrade requires the smallest possible code change to keep `go test ./...` green.

## Forbidden Paths

- `docker-compose.yml`
- `.env.example`
- `.golangci.yml`
- `.github/agents/AGENTS.md`
- `migrations/**`
- `local-env/**`
- `e2e-test.sh`
- `README.md`

## Known Abstraction Opportunities

- none

## Allowed Abstraction Scope

None. Edits stay at the file-local minimum required to satisfy the FRs.

## Required Tests

- `go fmt ./...` produces no diff.
- `go vet ./...` exits clean.
- `go build ./...` succeeds.
- `go test ./...` passes.
- `golangci-lint run ./...` runs (using the configuration added in step-001). Findings, if any, are reported.
- `docker build .` succeeds against the new builder image. (Run only if Docker is available on the implementation host; otherwise report it as not run.)

Add or update Go tests if and only if a dependency upgrade changes observable behavior the existing tests do not cover. The default expectation is that no test changes are required.

## Coverage Requirement

100% on any executable line changed in this step. Repository coverage must remain above 90%. If a dependency upgrade forces a code change, the change MUST be covered by a test in the same step.

## Failure Model

Fail-fast. Do not silently downgrade a dependency to dodge a test failure. Do not pin the builder image to a non-`1.26.2-*` tag. Do not introduce a `replace` directive to mask an upgrade problem. If a dependency upgrade breaks the build or tests in a way that cannot be fixed within a few lines, stop and surface the breakage as a blocker.

## Allowed Fallbacks

- none

## Acceptance Criteria

- `go.mod` declares `go 1.26.2`.
- A `toolchain` directive, when present, names exactly `go1.26.2`.
- `Dockerfile` builder stage line reads exactly `FROM golang:1.26.2-bookworm AS build`.
- No file in the allowed write paths references `golang:1.25`, `golang:1.26` (without patch), `golang:1`, or `golang:latest`.
- Each direct `require` entry in `go.mod` is at the latest stable version available at implementation time and pinned exactly.
- `go mod tidy` produces no further diff after the upgrade.
- All required validation commands listed above have been run, with results recorded in the completion report. Any command not run (e.g., Docker unavailable) is explicitly reported with the reason.

## Deferred Work

- Compose-side hardening of `POSTGRES_PASSWORD` is deferred to step-003.
- `AGENTS.md` verification is deferred to step-004.

## Escalation Conditions

- The `golang:1.26.2-bookworm` tag cannot be pulled.
- A direct dependency upgrade breaks `go build ./...` or `go test ./...` and the fix would expand scope beyond a few lines.
- `go mod tidy` keeps producing diffs after a clean run, indicating a deeper module-graph problem.
