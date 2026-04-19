# Local Environment & Toolchain Hardening

## Metadata
- Feature: local-env-fixes
- Status: ready-for-implementation
- Owner: SpecStar
- Last Updated: 2026-04-19
- Source Request: Harden the Go platform baseline so coding agents stop drifting on toolchain, linting, and local runtime setup. Pin Go 1.26.2, add repository-owned `golangci-lint` config, fix the Docker Compose PostgreSQL password bootstrap, and finalize `AGENTS.md` as a fully repository-specific contract.

## Problem Statement

The repository has four hardening gaps that cause coding agents and developers to drift away from the intended baseline:

1. `Dockerfile` builds with `golang:1.25-bookworm`, contradicting the repository standard of Go 1.26.2.
2. `go.mod` declares `go 1.25.0`, also contradicting the repository standard.
3. `docker-compose.yml` reads `POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}` with no default. On a clean machine without `.env`, the variable resolves to an empty string and the official `postgres` image refuses to start a database with an empty superuser password, breaking `docker compose up`.
4. There is no checked-in `.golangci.yml` / `.golangci.yaml`, even though `AGENTS.md` declares `golangci-lint` to be the primary static-analysis gate and requires a repository-owned configuration.

`AGENTS.md` itself was already substantially patched in `.github/agents/AGENTS.md` (Go 1.26.2 pinned, golangci-lint required, postgres password required, `go build ./...` listed, no remaining placeholder sections). Its remaining work is a verification pass to confirm no drift was reintroduced — not a rewrite.

## Goal

Make the repository deterministic for future agent-driven changes by:

1. Pinning the Go toolchain to **1.26.2** in every repository-owned reference.
2. Pinning the builder image to **`golang:1.26.2-bookworm`**.
3. Adding a repository-owned `.golangci.yml` that becomes the source of truth for lint behavior.
4. Making `docker compose up` succeed on a clean machine by enforcing a non-empty `POSTGRES_PASSWORD` at compose-resolution time, fail-fast.
5. Verifying `AGENTS.md` matches the locked baseline above.
6. Updating direct Go module dependencies and the Lambda base image touched by this work to current stable versions, with exact pinning.

## Functional Requirements

### Toolchain pinning

- FR-1: `go.mod` declares `go 1.26.2` (replacing `go 1.25.0`). The `toolchain` directive, if present, names an exact `go1.26.2` toolchain.
- FR-2: `Dockerfile` builder stage uses `FROM golang:1.26.2-bookworm AS build`. No floating tag (`golang:1`, `golang:1.26`, `latest`) and no `1.25.x` tag.
- FR-3: No repository-owned file under `cmd/`, `internal/`, `Dockerfile`, `docker-compose.yml`, `e2e-test.sh`, `README.md`, `migrations/`, `local-env/`, `.github/agents/AGENTS.md`, or `.github/instructions/` references Go 1.25.x after the change. Existing per-feature design notes under `.agent-specstar/features/<other-feature>/` are historical artifacts and are out of scope.

### `.golangci.yml`

- FR-4: A `.golangci.yml` exists at the repository root.
- FR-5: It targets the `golangci-lint` v2 schema (`version: "2"`), which is the current major series.
- FR-6: It enables at minimum: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`. Additional linters MUST be justified in the file as a brief comment if added; otherwise the baseline stays minimal.
- FR-7: It does not disable any of the five required linters via `linters.disable`, exclusion rules, or per-file overrides.
- FR-8: It does not introduce speculative excludes for repository code; `golangci-lint` defaults for excluding generated files and vendored code are acceptable.
- FR-9: `golangci-lint run ./...` exits with a non-zero status only on real findings, not on configuration errors.

### Docker Compose PostgreSQL bootstrap

- FR-10: `docker compose up --build` succeeds on a clean machine that has only Docker and Docker Compose v2 installed and a copy of `.env` derived from the committed `.env.example`.
- FR-11: If `POSTGRES_PASSWORD` is unset or empty when `docker compose` resolves variables, compose itself MUST fail fast with a clear error before any container starts. Use Docker Compose's required-variable interpolation form (e.g., `${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set and non-empty}`) for `POSTGRES_PASSWORD` and for any other variable whose absence would silently break the bootstrap (`POSTGRES_DB`, `POSTGRES_USER`).
- FR-12: `.env.example` continues to ship a non-empty default for `POSTGRES_PASSWORD` so the documented copy-and-run flow keeps working.
- FR-13: The `lambda` service's composed `DATABASE_URL` continues to use the same `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` values, so credentials cannot desync between the two services.
- FR-14: The bootstrap continues to mount `migrations/001_initial_schema.sql` and `local-env/init/02-seed.sql` into `/docker-entrypoint-initdb.d` unchanged, with the same numeric ordering.

### Lambda runtime image

- FR-15: The runtime stage continues to use `public.ecr.aws/lambda/provided:al2023`. If a newer immutable tag in the same `provided:al2023` line is verified at implementation time, it MAY be used; otherwise keep `al2023`. Do not switch to a different runtime family.

### Direct dependency refresh

- FR-16: Direct `require` entries in `go.mod` are upgraded to the latest stable versions available at implementation time, with exact pinning. Current direct dependencies in scope:
  - `github.com/aws/aws-lambda-go` (currently transitively listed as indirect; if the `cmd/http-lambda` entrypoint imports it, it must surface as a direct dependency.)
  - `github.com/google/uuid`
  - `github.com/jackc/pgx/v5`
  - `github.com/shopspring/decimal`
- FR-17: `go mod tidy` runs cleanly after the upgrade.
- FR-18: No new direct dependencies are introduced. Indirect dependencies move only as a side-effect of upgrading the four direct ones above.
- FR-19: All existing tests (`go test ./...`) pass after the upgrade. If a dependency upgrade requires a non-trivial code change, that change is added in the same step and stays narrow.

### `AGENTS.md` verification

- FR-20: `.github/agents/AGENTS.md` is reviewed against this design. Required content (already present today) is confirmed:
  - repository purpose describes a fixed-income wallet position service
  - Go 1.26.2 is the pinned toolchain version
  - builder image is pinned to `golang:1.26.2-bookworm`
  - local harness is Docker Compose + PostgreSQL + RIE-based local Lambda execution
  - PostgreSQL must define a non-empty `POSTGRES_PASSWORD`
  - `golangci-lint` requires a repository-owned configuration
  - validation commands include `go build ./...`
  - no remaining placeholder/template sections
- FR-21: Any drift discovered during verification is corrected in the same step. If no drift is found, the file is left unchanged and the verification result is recorded in the step's completion report.

## Non-Functional Requirements

- NFR-1: Determinism: every toolchain, image, and lint configuration choice owned by the repository is exact-pinned.
- NFR-2: Lightweight: no new heavy frameworks, lint plugins, or developer tooling layers are introduced.
- NFR-3: Fail-fast: missing or empty critical environment variables cause immediate, explicit failure rather than producing a broken running stack.
- NFR-4: Reversibility: each change is local to a single file or a small group of related files and can be reviewed in isolation.

## Scope

### In Scope

- `go.mod` and `go.sum` (Go directive, toolchain directive if applicable, direct dependency versions).
- `Dockerfile` (builder image tag).
- `docker-compose.yml` (required-variable interpolation for `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`).
- `.env.example` (kept aligned; non-empty defaults preserved).
- `.golangci.yml` (new).
- `.github/agents/AGENTS.md` (verification pass; corrections only if drift is found).
- Validation runs of `go fmt`, `go vet`, `go build`, `go test`, `golangci-lint run`, and a Docker Compose smoke check.

### Out of Scope

- Business-domain behavior changes for deposit or withdrawal flows.
- Architectural redesign of the service.
- Replacing the local RIE-based Lambda harness with SAM, LocalStack, or another harness.
- Production deployment or infrastructure-as-code changes.
- Observability redesign beyond toolchain/config hardening.
- Rewriting historical per-feature design notes under `.agent-specstar/features/` to align with Go 1.26.2.
- Introducing new direct dependencies, new linters beyond the required minimum without justification, or new repository-wide formatting rules.

## Constraints and Assumptions

- Repository-standard Go version is **Go 1.26.2**.
- Builder image MUST be `golang:1.26.2-bookworm` unless the implementation step explicitly proves another exact `1.26.2-*` image is better. No such proof is anticipated.
- `golangci-lint` v2 is the targeted major series for the configuration schema. The implementation step verifies the installed version matches this assumption before finalizing the file.
- Docker Compose v2 syntax with required-variable interpolation (`${VAR:?message}`) is supported by every Compose v2 release.
- `.env` is not committed; only `.env.example` is.
- The repository follows fail-fast, error-handling, logging, observability, testing, and cleanup instructions defined under `.github/instructions/`.
- No new Go dependency is introduced. Direct dependencies move only to current stable versions.

## Existing Context

- `.github/agents/AGENTS.md` — repository contract for coding agents; already aligned with most of this feature's requirements. Verification only.
- `.github/instructions/` — cross-cutting rules (cleanup, error-handling, fail-fast, logging, observability, testing) that apply to every change in this feature.
- `Dockerfile` — multi-stage build. Only the builder `FROM` line changes.
- `docker-compose.yml` — two-service compose (`postgres`, `lambda`). Only the variable interpolation form for credential variables changes.
- `.env.example` — already provides `POSTGRES_PASSWORD=wallet`. Unchanged unless a new key is required (none anticipated).
- `go.mod` — Go directive and direct `require` block change.
- `migrations/001_initial_schema.sql`, `local-env/init/02-seed.sql`, `e2e-test.sh`, `README.md` — unchanged by this feature unless validation surfaces a contradiction.

## Technical Approach

### Toolchain

- Replace `go 1.25.0` in `go.mod` with `go 1.26.2`.
- If the local Go toolchain in the implementation environment is exactly 1.26.2, no `toolchain` directive is required. If a different patch version is present, add an explicit `toolchain go1.26.2` directive so module operations are deterministic.
- Run `go mod tidy` after the change.

### Builder image

- Replace `FROM golang:1.25-bookworm AS build` with `FROM golang:1.26.2-bookworm AS build` in `Dockerfile`.
- Leave the rest of the multi-stage layout unchanged. The runtime stage continues to use `public.ecr.aws/lambda/provided:al2023`.

### Docker Compose required variables

For every variable whose absence or empty value would silently break the local stack, switch to the required-variable interpolation form:

```yaml
POSTGRES_DB: ${POSTGRES_DB:?POSTGRES_DB must be set and non-empty}
POSTGRES_USER: ${POSTGRES_USER:?POSTGRES_USER must be set and non-empty}
POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set and non-empty}
```

Apply the same form inside the composed `DATABASE_URL` for the `lambda` service. The `${VAR:-default}` form is explicitly forbidden for these credentials because a silent default would re-introduce the bug class this feature is closing.

`POSTGRES_HOST_PORT` and `LAMBDA_HOST_PORT` may keep their `${VAR:-N}` defaults; they are non-sensitive ergonomics and have safe defaults documented in `.env.example`.

### `.golangci.yml`

Minimal configuration targeting the v2 schema:

```yaml
version: "2"

linters:
  default: none
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
```

Rationale lives in this design file, not as a free-form comment block in the YAML. The implementation step is allowed to add up to one short comment line tying the file back to `AGENTS.md` if it improves discoverability, but is not required to.

If the installed `golangci-lint` reports the v2 schema is incompatible (e.g., because the installed binary is v1), the implementation step MUST stop and surface this as a blocker rather than silently downgrading the schema.

### Direct dependency refresh

- Run `go get -u <pkg>@latest` for each of the four direct dependencies, then `go mod tidy`.
- If `aws-lambda-go` is imported directly by `cmd/http-lambda/main.go`, it must appear in the direct `require` block after `go mod tidy`.
- Re-run `go test ./...` after the upgrade. If any test fails purely due to the upgrade, fix it within the same step with the smallest possible change. If a fix would expand scope beyond a few lines, stop and surface a clarification.

### `AGENTS.md` verification

- Diff `.github/agents/AGENTS.md` against the requirements list in FR-20.
- If every requirement is already satisfied, leave the file unchanged and record the verification outcome in the step's completion report.
- If any requirement drifted, correct it with the smallest possible edit.

## Affected Components

- `go.mod`, `go.sum`
- `Dockerfile`
- `docker-compose.yml`
- `.env.example` (verification; no change anticipated)
- `.golangci.yml` (new)
- `.github/agents/AGENTS.md` (verification; correction only if drift is found)

## Contracts and Data Shape Impact

None. No HTTP, SQS, DTO, schema, or persistence contracts change.

## State / Persistence Impact

None at runtime. PostgreSQL data layout, schema, and seed scripts are untouched.

## Failure Model

- Compose-time: missing `POSTGRES_DB`, `POSTGRES_USER`, or `POSTGRES_PASSWORD` causes Docker Compose to abort variable resolution with the configured error message before any container is created.
- Build-time: a non-existent `golang:1.26.2-bookworm` tag (not anticipated) causes `docker build` to fail at image pull; the implementation step does not invent a fallback tag.
- Lint-time: an incompatible `golangci-lint` major version causes `golangci-lint run` to fail with a configuration error; the implementation step surfaces this as a blocker rather than altering the v2 schema to match an older binary.
- Test-time: a dependency upgrade that breaks `go test ./...` halts the step until the breakage is fixed within the approved scope or escalated.

No fail-safe fallbacks are added in any of the above. Each failure mode is intentional fail-fast.

## Testing and Validation Strategy

For every step that changes Go code or module configuration:

```bash
go fmt ./...
go vet ./...
go build ./...
go test ./...
golangci-lint run ./...
```

For every step that changes `Dockerfile` or `docker-compose.yml`:

```bash
docker compose config            # variable resolution sanity check
docker compose up --build -d
docker compose ps
docker compose down
```

Each step's completion report MUST list which validation commands were actually run, their outcomes, and any commands that could not be run together with the reason.

If `golangci-lint` is not available on the implementation host, the step MUST report that explicitly, run the remaining commands, and not pretend the lint gate passed.

If Docker is not available, the same rule applies to the Docker validation block.

## Execution Notes

- The hardening changes are mostly small, file-local edits. Resist the temptation to broaden scope into formatting, refactoring, or rewriting unrelated configuration during these steps.
- Do not introduce a `Makefile`, task runner, or new developer-facing tooling as part of this feature.
- Do not migrate `AGENTS.md` to a different location as part of this feature; it stays at `.github/agents/AGENTS.md`.
- Step ordering is intentional: lint config first (so the toolchain bump can be linted), then toolchain + image + dependency refresh, then compose hardening, then `AGENTS.md` verification. Compose hardening is intentionally late so the Docker validation can also exercise the new builder image.

## Open Questions

None blocking. All ambiguities are resolved by the FRs above.

## Success Criteria

- `go.mod` declares `go 1.26.2` and `go mod tidy` is clean.
- `Dockerfile` builder stage is `FROM golang:1.26.2-bookworm AS build`.
- `.golangci.yml` exists, targets the v2 schema, and enables at least the five required linters.
- `golangci-lint run ./...` runs and reports findings (or none) without configuration errors.
- `docker compose up --build` succeeds on a clean machine with `.env` copied from `.env.example`.
- `docker compose up` fails fast with the configured error message when `POSTGRES_PASSWORD` is unset or empty.
- `.github/agents/AGENTS.md` matches the locked baseline (FR-20). Drift, if any, is corrected.
- All direct dependencies are pinned to current stable versions and `go test ./...` passes.
- Required validation commands have been run and reported.
