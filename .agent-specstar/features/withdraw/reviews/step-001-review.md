# Step 001 Review — Application layer: WithdrawService

**Verdict: APPROVED**

**Reviewed files:**
- `internal/application/withdraw_service.go`
- `internal/application/withdraw_service_test.go`

**Reference files inspected:**
- `internal/application/deposit_service.go`
- `internal/application/deposit_service_test.go`
- `internal/domain/position.go`
- `internal/domain/errors.go`
- `internal/domain/processed_command.go`
- `internal/ports/position_repository.go`

---

## Review Criteria Assessment

### 1. Full 13-step orchestration flow from design.md

All 13 steps are implemented in order:

| # | Step | Location | Status |
|---|---|---|---|
| 1 | Parse and validate input | Lines 85–88 → `validateWithdrawRequest` | ✅ |
| 2 | Idempotency check | Lines 91–97 | ✅ |
| 3 | Client lookup | Lines 100–106 | ✅ |
| 4 | Find lots | Lines 109–112 | ✅ |
| 5 | No lots → 404 | Lines 115–117 | ✅ |
| 6 | Pass 1 — sufficiency check (read-only) | Lines 120–128 | ✅ |
| 7 | Insufficient position → 409 | Lines 131–133 | ✅ |
| 8 | `if_match` validation (between pass 1 and pass 2) | Lines 136–148 | ✅ |
| 9 | Pass 2 — mutation | Lines 151–163 | ✅ |
| 10 | Build WithdrawResponse | Lines 166–174 | ✅ |
| 11 | Atomic persistence (UnitOfWork.Do) | Lines 177–194 | ✅ |
| 12 | Handle ErrDuplicate (race) | Lines 197–199 | ✅ |
| 13 | Handle ErrConcurrencyConflict | Lines 201–203 | ✅ |

Client lookup and lot fetching happen outside the transaction. Position updates and ProcessedCommand creation happen inside `UnitOfWork.Do`. Matches design requirement.

### 2. Validation rules enforced in order, short-circuiting on first failure

`validateWithdrawRequest` (lines 242–281) enforces all 7 input rules in the correct order specified by the design validation table (rules 1–7). Each rule returns immediately on failure. Rule 8 (client exists) is enforced in `Execute` at lines 100–106, after validation and idempotency. ✅

### 3. Idempotency check before entity lookups

Idempotency check (line 91) precedes client lookup (line 100) and lot fetching (line 109). ✅

### 4. Two-pass lot selection algorithm

- **Pass 1** (lines 120–128): Read-only. Iterates lots, filters by `AvailableValue().IsPositive()`, accumulates `totalAvailable`, builds `eligibleLots`. No mutation. ✅
- **Pass 2** (lines 151–163): Mutates. Iterates `eligibleLots` in same FIFO order, computes `valueFromLot`, `unitsSold`, calls `UpdateAmount`. ✅

### 5. `if_match` validation between pass 1 and pass 2

Lines 136–148 sit correctly between pass 1 (lines 120–133) and pass 2 (lines 151–163). The `if_match` loop mirrors the pass 2 logic (same FIFO order, same `min(remaining, available)` determination) to identify affected lots without mutation. ✅

### 6. `units_sold = (valueFromLot / lot.UnitPrice()).Round(6)`

Line 158: `unitsSold := valueFromLot.Div(lot.UnitPrice()).Round(6)`. shopspring/decimal's `Round()` uses round-half-away-from-zero semantics, which for positive values is equivalent to the half-up rounding specified by the design. ✅

### 7. Typed errors inspectable via `errors.As`

- `InsufficientPositionError` (lines 21–23): pointer receiver, returned as `*InsufficientPositionError` at line 132. ✅
- `ConcurrencyConflictError` (lines 25–28): pointer receiver, returned as `*ConcurrencyConflictError` at lines 145 and 202. ✅

Both verified in tests via `errors.As` (tests 10, 12, 17).

### 8. JSON tags match design.md response contract

All JSON tags on `WithdrawResponse` and `AffectedPosition` (lines 40–57) exactly match the response DTO specification in design.md (lines 67–85):

- `affected_positions`, `position_id`, `client_id`, `asset_id`, `amount`, `unit_price`, `total_value`, `collateral_value`, `judiciary_collateral_value`, `purchased_at`, `created_at`, `updated_at` — all present and correctly tagged. ✅

All decimal and UUID fields are `string` type. Timestamps use `time.RFC3339` formatting (lines 237–239). ✅

### 9. Test scenario coverage

All 31 required scenarios are covered, plus 3 additional coverage tests:

| # | Scenario | Test function | Status |
|---|---|---|---|
| 1 | Single lot fully consumed | `TestWithdraw_SingleLotFullyConsumed_Returns200` | ✅ |
| 2 | Multiple lots FIFO | `TestWithdraw_MultipleLotsFIFO_Returns200` | ✅ |
| 3 | Partial last lot | `TestWithdraw_PartialLastLot_Returns200` | ✅ |
| 4 | AffectedPosition fields match | `TestWithdraw_AffectedPositionFieldsMatch` | ✅ |
| 5 | Non-positive available skipped | `TestWithdraw_LotsWithNonPositiveAvailableValueSkipped` | ✅ |
| 6 | Zero amount lots skipped | `TestWithdraw_ZeroAmountLotsSkipped` | ✅ |
| 7 | FIFO order preserved | `TestWithdraw_FIFOOrderPreserved` | ✅ |
| 8 | units_sold rounding | `TestWithdraw_UnitsSoldRounding` | ✅ |
| 9 | if_match all match | `TestWithdraw_IfMatchAllLotsMatch_Proceeds` | ✅ |
| 10 | if_match mismatch | `TestWithdraw_IfMatchMismatch_Returns409` | ✅ |
| 11 | if_match not provided | `TestWithdraw_IfMatchNotProvided_Proceeds` | ✅ |
| 12 | Insufficient position | `TestWithdraw_InsufficientPosition_Returns409` | ✅ |
| 13 | No lots → 404 | `TestWithdraw_NoLots_Returns404` | ✅ |
| 14 | Idempotent replay | `TestWithdraw_IdempotentReplay_Returns200` | ✅ |
| 15 | Race → re-read succeeds | `TestWithdraw_RaceCondition_ReplayReturns200` | ✅ |
| 16 | Race → re-read fails | `TestWithdraw_RaceCondition_RereadFails_Returns409` | ✅ |
| 17 | Update concurrency conflict | `TestWithdraw_UpdateConcurrencyConflict_Returns409` | ✅ |
| 18 | Missing client_id | `TestWithdraw_MissingClientID_Returns422` | ✅ |
| 19 | Invalid client_id | `TestWithdraw_InvalidClientID_Returns422` | ✅ |
| 20 | Missing product_asset_id | `TestWithdraw_MissingProductAssetID_Returns422` | ✅ |
| 21 | Missing order_id | `TestWithdraw_MissingOrderID_Returns422` | ✅ |
| 22 | Missing desired_value | `TestWithdraw_MissingDesiredValue_Returns422` | ✅ |
| 23 | Invalid desired_value | `TestWithdraw_InvalidDesiredValue_Returns422` | ✅ |
| 24 | Zero desired_value | `TestWithdraw_ZeroDesiredValue_Returns422` | ✅ |
| 25 | Negative desired_value | `TestWithdraw_NegativeDesiredValue_Returns422` | ✅ |
| 26 | Invalid if_match | `TestWithdraw_InvalidIfMatch_Returns422` | ✅ |
| 27 | Client not found | `TestWithdraw_ClientNotFound_Returns422` | ✅ |
| 28 | Client unexpected error | `TestWithdraw_ClientFindUnexpectedError_Returns500` | ✅ |
| 29 | ProcessedCommand unexpected error | `TestWithdraw_ProcessedCommandFindUnexpectedError_Returns500` | ✅ |
| 30 | Position find unexpected error | `TestWithdraw_PositionFindUnexpectedError_Returns500` | ✅ |
| 31 | Update unexpected error | `TestWithdraw_UpdateUnexpectedError_Returns500` | ✅ |
| 32+ | Corrupted snapshot | `TestWithdraw_IdempotentReplay_CorruptedSnapshot_Returns500` | ✅ |
| 33+ | if_match break at zero remaining | `TestWithdraw_IfMatchBreaksWhenRemainingZero` | ✅ |
| 34+ | Nil UUID → NewProcessedCommand fails | `TestWithdraw_NilUUIDClientID_NewProcessedCommandFails_Returns500` | ✅ |

34 test functions total. All pass.

### 10. 100% line coverage

Confirmed via `go tool cover -func`:

| Function | Coverage |
|---|---|
| `Error` (InsufficientPositionError) | 100.0% |
| `Error` (ConcurrencyConflictError) | 100.0% |
| `NewWithdrawService` | 100.0% |
| `Execute` | 100.0% |
| `replayAfterRace` | 100.0% |
| `deserializeWithdrawSnapshot` | 100.0% |
| `toAffectedPosition` | 100.0% |
| `validateWithdrawRequest` | 100.0% |

All functions at 100% line coverage. ✅

### 11. Errors wrapped with `%w` for cause chain preservation

All errors from repository/infrastructure calls are wrapped with `%w`:

- Line 93: `"find processed command: %w"` ✅
- Line 105: `"find client: %w"` ✅
- Line 111: `"find positions: %w"` ✅
- Line 180: `"update position: %w"` ✅
- Line 186: `"new processed command: %w"` ✅
- Line 190: `"create processed command: %w"` ✅
- Line 204: `"unit of work: %w"` ✅
- Line 213: `"replay after race: %w"` ✅
- Line 221: `"unmarshal response snapshot: %w"` ✅

Validation errors and typed errors are created fresh (not wrapping), which is correct.

### 12. Follows deposit_service.go patterns

- Same `Execute(ctx, req) → (*Response, int, error)` signature pattern ✅
- Same validation → idempotency → entity lookup → business logic → UnitOfWork.Do flow ✅
- Same `replayAfterRace` private method pattern ✅
- Same `deserializeXxxSnapshot` helper pattern ✅
- Same `toXxx` DTO mapping helper pattern ✅
- Same constructor injection pattern ✅
- Same error wrapping conventions ✅
- Same `json.Marshal` error discard with justifying comment ✅
- Mock naming uses `w` prefix to avoid collision with deposit test mocks (same package) — appropriate ✅

---

## Scope Compliance

**Allowed write paths modified:** `withdraw_service.go`, `withdraw_service_test.go` — both within `internal/application/`. ✅

**Forbidden paths:** No modifications to `internal/domain/`, `internal/ports/`, `internal/adapters/`, `cmd/`. ✅

**No new packages, interfaces, or port definitions.** ✅

## Build Verification

- `go build ./...` — passes ✅
- `go vet ./...` — no warnings ✅
- `go test ./internal/application/ -run TestWithdraw` — all 34 tests pass ✅

## Clean Code Observations

- No dead code or stale comments.
- No duplicated logic — `toAffectedPosition` and `deserializeWithdrawSnapshot` are appropriately factored as private helpers.
- Discarded error on `UpdateAmount` (line 160) is justified by the comment and the step contract's explicit acknowledgment that pass 1 guarantees non-negative results.
- Discarded error on `json.Marshal` (line 174) is justified by the comment (all-string struct) and matches the deposit pattern.
- No silent fallbacks or guessed defaults.

## Deferred Work

None. The step file declares no deferred work, and the implementation leaves none.

---

**Result: APPROVED — no fix-step required.**
