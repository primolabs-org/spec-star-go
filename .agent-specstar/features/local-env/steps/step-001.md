# Step 001 - Multi-stage Lambda Dockerfile

## Metadata
- Feature: local-env
- Step: step-001
- Status: pending
- Depends On: none
- Last Updated: 2026-04-19

## Objective

Author a multi-stage `Dockerfile` (and a companion `.dockerignore`) at the repository root that builds the wallet Lambda image from the unmodified `cmd/http-lambda/main.go` entrypoint and produces a runtime image based on `public.ecr.aws/lambda/provided:al2023` containing only the `bootstrap` binary.

## In Scope

- New `Dockerfile` at the repository root implementing the build/runtime stages described in `.agent-specstar/features/local-env/design.md` § Technical Approach → Lambda image.
- New `.dockerignore` at the repository root excluding at minimum: `.git/`, `.github/`, `.agent-specstar/`, `coverage.html`, `local-env/`, `e2e-test.sh`, `README.md`, `docker-compose.yml`, `.env*`, IDE/OS metadata.

## Out of Scope

- `docker-compose.yml`, `.env.example`, SQL files, `e2e-test.sh`, `README.md`.
- Any change to Go source, `go.mod`, `go.sum`, or `migrations/`.
- Pushing or tagging the image to a registry.

## Required Reads

- `.agent-specstar/features/local-env/design.md` — sections: Technical Approach → Lambda image, Affected Components, Failure Model.
- `cmd/http-lambda/main.go` — confirms the build target package path.
- `go.mod` — confirms Go version and module path.

## Allowed Write Paths

- `Dockerfile`
- `.dockerignore`

## Forbidden Paths

- `cmd/**`
- `internal/**`
- `go.mod`, `go.sum`
- `migrations/**`
- `docker-compose.yml`
- `.env`, `.env.example`
- `local-env/**`
- `e2e-test.sh`
- `README.md`
- `.agent-specstar/**`

## Known Abstraction Opportunities

None. A multi-stage Dockerfile with one build and one runtime stage is the smallest viable form.

## Allowed Abstraction Scope

None beyond the two-stage layout. Do not introduce build args, base-image variables, or extra stages unless required to pass acceptance.

## Required Tests

This step produces no Go executable lines and is validated by image-level checks:

1. `docker build -t wallet-lambda:local .` from the repo root succeeds with no errors.
2. `docker image inspect wallet-lambda:local --format '{{.Config.ExposedPorts}}'` contains `8080/tcp` (inherited from the AWS base image).
3. `docker run --rm -d --name wallet-lambda-smoke -p 9000:8080 wallet-lambda:local` starts the RIE; an HTTP probe `curl -fsS -X POST -H 'Content-Type: application/json' -d '{}' http://localhost:9000/2015-03-31/functions/function/invocations` returns a 200-class HTTP status (the embedded Lambda payload may report any error because no DB is configured; the HTTP layer reaching the RIE is what matters). Container is then stopped with `docker rm -f wallet-lambda-smoke`.

These are manual or scripted shell checks; no Go tests are added.

## Coverage Requirement

This step changes **zero Go executable lines**. Go-side coverage rules do not apply. The Dockerfile is validated by the build and smoke checks listed under Required Tests; no Go coverage is collected.

## Failure Model

Fail-fast. The build must fail on any compile error, missing file, or invalid base-image reference. Do not introduce conditional `RUN` fallbacks, `|| true`, or retry loops.

## Allowed Fallbacks

- none

## Acceptance Criteria

1. `Dockerfile` exists at the repo root and uses two stages: a `golang:1.25-bookworm` (or pinned equivalent satisfying `go 1.25.0`) builder and a `public.ecr.aws/lambda/provided:al2023` runtime.
2. The builder stage caches `go mod download` in a layer separate from source `COPY` so source-only edits do not re-download dependencies.
3. The build command produces a static Linux/amd64 binary named `bootstrap` using `CGO_ENABLED=0`, `GOOS=linux`, `GOARCH=amd64`, `-trimpath`, and `-ldflags="-s -w"`, targeting `./cmd/http-lambda`.
4. The runtime stage copies the binary to `${LAMBDA_RUNTIME_DIR}/bootstrap` and sets a non-empty `CMD` per the `provided.al2023` contract.
5. `.dockerignore` excludes the paths listed under In Scope so the build context stays small and unrelated files (notably the local-env folder, the e2e script, and `.agent-specstar/`) are never sent to the daemon.
6. The smoke checks under Required Tests pass on a clean machine.
7. No file outside Allowed Write Paths is modified.

## Deferred Work

- none

## Escalation Conditions

- The `public.ecr.aws/lambda/provided:al2023` image is unavailable from the developer's network. Stop and escalate; do not substitute a different base image without an updated design.
- The `golang:1.25-bookworm` tag does not exist at execution time. Escalate to confirm a pinned alternative that still satisfies `go 1.25.0`.
