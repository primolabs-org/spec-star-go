# Clarifications — domain

## Open decisions requiring resolution before or during implementation

### 1. Decimal library approval

**Status**: Blocking for entity implementation

`shopspring/decimal` is recommended for exact decimal arithmetic (`NUMERIC(18,6)` and `NUMERIC(20,8)` fields). It is not in the pre-approved library list.

**Options**:
- **Approve `shopspring/decimal`** — de facto standard for Go financial systems, minimal footprint, maps cleanly to PostgreSQL `NUMERIC`.
- **Use `int64` fixed-point with `math/big` fallback** — standard library only, correct but verbose and error-prone for every arithmetic operation.

### 2. UUID generation strategy

**Status**: Blocking for entity construction

The schema uses UUID primary keys. UUIDs need to come from somewhere.

**Options**:
- **Approve `google/uuid`** — lightweight, widely used, standard for Go UUID generation.
- **Caller-provided UUIDs** — the domain accepts pre-generated UUID strings, no external library needed. Validation of UUID format uses standard library only.

### 3. Migration runner

**Status**: Non-blocking for step generation

Plain numbered SQL files are generated. How they are applied needs clarification.

**Options**:
- **External CI/CD tool** — migrations applied outside the Go application.
- **Minimal Go bootstrap utility** — thin migration runner in `internal/platform/` or `cmd/migrate/`.
- **Manual execution** — developer runs SQL files against the database directly.

### 4. `response_snapshot` typing

**Status**: Non-blocking for this feature (commands don't exist yet)

**Options**:
- **Opaque `[]byte` (JSON)** — domain stores raw JSON, command implementations define structure later.
- **Typed per command** — domain defines a structured type per command, stricter but more coupling.

**Recommendation**: Opaque `[]byte` for now since no commands exist in this feature.
