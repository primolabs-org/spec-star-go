# Step 002 Review — Application layer: deposit service

## Verdict: APPROVED

## Scope compliance

- Only allowed write paths were touched: `internal/application/deposit_service.go` and `internal/application/deposit_service_test.go`.
- No forbidden paths modified (`internal/domain/`, `internal/ports/`, `internal/adapters/`, `cmd/`).
- No scope creep detected.

## Acceptance criteria

| # | Criterion | Status |
|---|---|---|
| 1 | Full orchestration flow per design.md | Pass |
| 2 | Validation rules 2–11 enforced in order, short-circuit on first failure | Pass |
| 3 | Idempotency check before entity lookups | Pass |
| 4 | Client/Asset lookups outside UnitOfWork.Do | Pass |
| 5 | Position/ProcessedCommand creation inside UnitOfWork.Do | Pass |
| 6 | DepositResponse JSON tags match response contract | Pass |
| 7 | Decimal fields serialize as strings | Pass |
| 8 | purchased_at set to time.Now().UTC() | Pass |
| 9 | All test scenarios pass | Pass (27/27) |
| 10 | go build ./... succeeds | Pass |
| 11 | go vet ./... clean | Pass |

## Coverage

100% line coverage on `deposit_service.go` across all six functions: `NewDepositService`, `Execute`, `replayAfterRace`, `deserializeSnapshot`, `toDepositResponse`, `validateDepositRequest`.

## Test inventory

27 test functions cover all 25 required scenarios plus 2 additional for full coverage:

### Happy path (2)
1. `TestExecute_ValidDeposit_Returns201` — scenario 1
2. `TestExecute_ValidDeposit_ResponseFieldsMatchPosition` — scenario 2

### Idempotency replay (1)
3. `TestExecute_IdempotentReplay_Returns200` — scenario 3 (also verifies no Create calls)

### Idempotency concurrent race (2)
4. `TestExecute_RaceCondition_ReplayReturns200` — scenario 4
5. `TestExecute_RaceCondition_RereadFails_Returns409` — scenario 5

### Validation failures (13)
6–18. All 13 validation rules tested with correct error messages and 422 status.

### Entity lookup failures (3)
19. `TestExecute_ClientNotFound_Returns422` — scenario 19
20. `TestExecute_AssetNotFound_Returns422` — scenario 20
21. `TestExecute_UnsupportedProductType_Returns422` — scenario 21

### Unexpected errors (4)
22. `TestExecute_ClientFindUnexpectedError_Returns500` — scenario 22
23. `TestExecute_AssetFindUnexpectedError_Returns500` — scenario 23
24. `TestExecute_ProcessedCommandFindUnexpectedError_Returns500` — scenario 24
25. `TestExecute_PositionCreateUnexpectedError_Returns500` — scenario 25

### Additional for coverage (2)
26. `TestExecute_NilUUIDClientID_NewProcessedCommandFails_Returns500` — covers `domain.NewProcessedCommand` error path inside UnitOfWork
27. `TestExecute_IdempotentReplay_CorruptSnapshot_Returns500` — covers `deserializeSnapshot` unmarshal failure

## Error handling

- All errors propagated with `fmt.Errorf("...: %w", err)` preserving cause chain.
- No swallowed errors. Two justified `_` assignments with inline comments:
  - `domain.NewPosition` cannot fail because the service validates `> 0` before calling it (domain only rejects negative).
  - `json.Marshal` on a struct of only string fields cannot fail.
- `ErrDuplicate` race resolution is the only recovery path, explicitly required by design.

## Clean code

- Cognitive complexity within limits; `Execute` uses guard clauses and early returns.
- `toDepositResponse` helper avoids duplication per step's Known Abstraction Opportunities.
- `commandTypeDeposit` constant avoids magic string.
- No dead code, stale comments, unused imports, or temporary artifacts.
- Mock implementations satisfy full port interfaces.

## Deferred work

None. Step declares no deferred work; nothing is left uncovered.
