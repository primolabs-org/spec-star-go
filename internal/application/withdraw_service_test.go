package application

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/shopspring/decimal"
)

// --- Withdraw mock implementations ---

type wMockClientRepo struct {
	findByIDFn func(ctx context.Context, clientID uuid.UUID) (*domain.Client, error)
}

func (m *wMockClientRepo) FindByID(ctx context.Context, clientID uuid.UUID) (*domain.Client, error) {
	return m.findByIDFn(ctx, clientID)
}

func (m *wMockClientRepo) Create(_ context.Context, _ *domain.Client) error { return nil }

type wMockPositionRepo struct {
	findByClientAndInstrumentFn func(ctx context.Context, clientID uuid.UUID, instrumentID string) ([]*domain.Position, error)
	updateFn                    func(ctx context.Context, position *domain.Position) error
}

func (m *wMockPositionRepo) FindByID(_ context.Context, _ uuid.UUID) (*domain.Position, error) {
	return nil, nil
}

func (m *wMockPositionRepo) FindByClientAndAsset(_ context.Context, _, _ uuid.UUID) ([]*domain.Position, error) {
	return nil, nil
}

func (m *wMockPositionRepo) FindByClientAndInstrument(ctx context.Context, clientID uuid.UUID, instrumentID string) ([]*domain.Position, error) {
	return m.findByClientAndInstrumentFn(ctx, clientID, instrumentID)
}

func (m *wMockPositionRepo) Create(_ context.Context, _ *domain.Position) error { return nil }

func (m *wMockPositionRepo) Update(ctx context.Context, position *domain.Position) error {
	return m.updateFn(ctx, position)
}

type wMockProcCmdRepo struct {
	findByTypeAndOrderIDFn func(ctx context.Context, commandType, orderID string) (*domain.ProcessedCommand, error)
	createFn               func(ctx context.Context, cmd *domain.ProcessedCommand) error
}

func (m *wMockProcCmdRepo) FindByTypeAndOrderID(ctx context.Context, commandType, orderID string) (*domain.ProcessedCommand, error) {
	return m.findByTypeAndOrderIDFn(ctx, commandType, orderID)
}

func (m *wMockProcCmdRepo) Create(ctx context.Context, cmd *domain.ProcessedCommand) error {
	return m.createFn(ctx, cmd)
}

type wMockUoW struct {
	doFn func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (m *wMockUoW) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.doFn(ctx, fn)
}

// --- Test helpers ---

func validWithdrawRequest() WithdrawRequest {
	return WithdrawRequest{
		ClientID:       uuid.New().String(),
		ProductAssetID: "INST-001",
		OrderID:        "order-w-123",
		DesiredValue:   "100",
	}
}

func newWithdrawTestClient() *domain.Client {
	client, _ := domain.NewClient("ext-w-001")
	return client
}

func buildWithdrawService(
	clients *wMockClientRepo,
	positions *wMockPositionRepo,
	processedCmds *wMockProcCmdRepo,
	uow *wMockUoW,
) *WithdrawService {
	return NewWithdrawService(clients, positions, processedCmds, uow)
}

func withdrawDefaultMocks() (*wMockClientRepo, *wMockPositionRepo, *wMockProcCmdRepo, *wMockUoW) {
	clientID := uuid.New()
	assetID := uuid.New()
	now := time.Now().UTC()
	purchased := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	lot, _ := domain.ReconstructPosition(
		uuid.New(), clientID, assetID,
		decimal.NewFromInt(100), decimal.NewFromInt(10), decimal.NewFromInt(1000),
		decimal.Zero, decimal.Zero,
		now, now, purchased, 1,
	)

	clients := &wMockClientRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Client, error) {
			return newWithdrawTestClient(), nil
		},
	}
	positions := &wMockPositionRepo{
		findByClientAndInstrumentFn: func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
			return []*domain.Position{lot}, nil
		},
		updateFn: func(_ context.Context, _ *domain.Position) error {
			return nil
		},
	}
	processedCmds := &wMockProcCmdRepo{
		findByTypeAndOrderIDFn: func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
			return nil, domain.ErrNotFound
		},
		createFn: func(_ context.Context, _ *domain.ProcessedCommand) error {
			return nil
		},
	}
	uow := &wMockUoW{
		doFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	}
	return clients, positions, processedCmds, uow
}

func makePosition(clientID, assetID uuid.UUID, amount, unitPrice decimal.Decimal, collateral, judiciaryCollateral decimal.Decimal, purchased time.Time, rowVersion int) *domain.Position {
	totalValue := amount.Mul(unitPrice)
	now := time.Now().UTC()
	p, _ := domain.ReconstructPosition(
		uuid.New(), clientID, assetID,
		amount, unitPrice, totalValue,
		collateral, judiciaryCollateral,
		now, now, purchased, rowVersion,
	)
	return p
}

// --- Happy path tests ---

// Test 1: Valid withdrawal consuming a single lot fully.
func TestWithdraw_SingleLotFullyConsumed_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	purchased := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	lot := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, purchased, 1)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "100" // 10 * 10 = 100 total value

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.AffectedPositions) != 1 {
		t.Fatalf("expected 1 affected position, got %d", len(resp.AffectedPositions))
	}
	if resp.AffectedPositions[0].Amount != "0" {
		t.Errorf("expected amount 0, got %s", resp.AffectedPositions[0].Amount)
	}
}

// Test 2: Valid withdrawal consuming multiple lots in FIFO order.
func TestWithdraw_MultipleLotsFIFO_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	lot1 := makePosition(clientID, assetID, decimal.NewFromInt(5), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 1)
	lot2 := makePosition(clientID, assetID, decimal.NewFromInt(5), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), 1)
	lot3 := makePosition(clientID, assetID, decimal.NewFromInt(5), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), 1)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot1, lot2, lot3}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "100" // lot1: 50, lot2: 50 → exactly 100

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if len(resp.AffectedPositions) != 2 {
		t.Fatalf("expected 2 affected positions, got %d", len(resp.AffectedPositions))
	}
	// First lot fully consumed
	if resp.AffectedPositions[0].Amount != "0" {
		t.Errorf("expected first lot amount 0, got %s", resp.AffectedPositions[0].Amount)
	}
	// Second lot fully consumed
	if resp.AffectedPositions[1].Amount != "0" {
		t.Errorf("expected second lot amount 0, got %s", resp.AffectedPositions[1].Amount)
	}
}

// Test 3: Valid withdrawal partially consuming the last lot.
func TestWithdraw_PartialLastLot_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	lot1 := makePosition(clientID, assetID, decimal.NewFromInt(5), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 1)
	lot2 := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), 1)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot1, lot2}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "70" // lot1: 50 fully consumed, lot2: 20 partial → 2 units sold from lot2

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if len(resp.AffectedPositions) != 2 {
		t.Fatalf("expected 2 affected positions, got %d", len(resp.AffectedPositions))
	}
	// First lot fully consumed
	if resp.AffectedPositions[0].Amount != "0" {
		t.Errorf("expected first lot amount 0, got %s", resp.AffectedPositions[0].Amount)
	}
	// Second lot partially consumed: 10 - (20/10) = 10 - 2 = 8
	if resp.AffectedPositions[1].Amount != "8" {
		t.Errorf("expected second lot amount 8, got %s", resp.AffectedPositions[1].Amount)
	}
}

// Test 4: Verify AffectedPosition field values match the mutated Position state.
func TestWithdraw_AffectedPositionFieldsMatch(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	purchased := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	lot := makePosition(clientID, assetID, decimal.NewFromInt(20), decimal.RequireFromString("5.5"), decimal.Zero, decimal.Zero, purchased, 1)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "11" // units_sold = (11 / 5.5).Round(6) = 2, new_amount = 20 - 2 = 18

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	ap := resp.AffectedPositions[0]
	if ap.PositionID != lot.PositionID().String() {
		t.Errorf("expected position_id %s, got %s", lot.PositionID().String(), ap.PositionID)
	}
	if ap.ClientID != lot.ClientID().String() {
		t.Errorf("expected client_id %s, got %s", lot.ClientID().String(), ap.ClientID)
	}
	if ap.AssetID != lot.AssetID().String() {
		t.Errorf("expected asset_id %s, got %s", lot.AssetID().String(), ap.AssetID)
	}
	if ap.Amount != "18" {
		t.Errorf("expected amount 18, got %s", ap.Amount)
	}
	if ap.UnitPrice != "5.5" {
		t.Errorf("expected unit_price 5.5, got %s", ap.UnitPrice)
	}
	// totalValue = 18 * 5.5 = 99
	if ap.TotalValue != "99" {
		t.Errorf("expected total_value 99, got %s", ap.TotalValue)
	}
	if ap.CollateralValue != "0" {
		t.Errorf("expected collateral_value 0, got %s", ap.CollateralValue)
	}
	if ap.JudiciaryCollateralValue != "0" {
		t.Errorf("expected judiciary_collateral_value 0, got %s", ap.JudiciaryCollateralValue)
	}
	if ap.PurchasedAt != purchased.Format(time.RFC3339) {
		t.Errorf("expected purchased_at %s, got %s", purchased.Format(time.RFC3339), ap.PurchasedAt)
	}
	if _, err := time.Parse(time.RFC3339, ap.CreatedAt); err != nil {
		t.Errorf("created_at is not valid RFC3339: %s", ap.CreatedAt)
	}
	if _, err := time.Parse(time.RFC3339, ap.UpdatedAt); err != nil {
		t.Errorf("updated_at is not valid RFC3339: %s", ap.UpdatedAt)
	}
}

// --- Lot selection edge cases ---

// Test 5: Lots with AvailableValue() <= 0 are skipped.
func TestWithdraw_LotsWithNonPositiveAvailableValueSkipped(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	// Lot with collateral = totalValue → available = 0
	blockedLot := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.NewFromInt(100), decimal.Zero, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 1)
	// Normal lot
	goodLot := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), 1)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{blockedLot, goodLot}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "50"

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	// Only the good lot should be affected
	if len(resp.AffectedPositions) != 1 {
		t.Fatalf("expected 1 affected position, got %d", len(resp.AffectedPositions))
	}
	if resp.AffectedPositions[0].PositionID != goodLot.PositionID().String() {
		t.Errorf("expected good lot to be affected, got %s", resp.AffectedPositions[0].PositionID)
	}
}

// Test 6: Lots with zero amount (fully depleted) are skipped.
func TestWithdraw_ZeroAmountLotsSkipped(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	// Depleted lot: amount=0
	depletedLot := makePosition(clientID, assetID, decimal.Zero, decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 1)
	// Normal lot
	goodLot := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), 1)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{depletedLot, goodLot}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "50"

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if len(resp.AffectedPositions) != 1 {
		t.Fatalf("expected 1 affected position, got %d", len(resp.AffectedPositions))
	}
	if resp.AffectedPositions[0].PositionID != goodLot.PositionID().String() {
		t.Errorf("expected good lot to be affected")
	}
}

// Test 7: FIFO order preserved (oldest purchased_at consumed first).
func TestWithdraw_FIFOOrderPreserved(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	oldLot := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 1)
	newLot := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), 1)

	// Repository returns in FIFO order (oldest first)
	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{oldLot, newLot}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "50" // Consumes from oldLot first (50 of its 100 available)

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if len(resp.AffectedPositions) != 1 {
		t.Fatalf("expected 1 affected position, got %d", len(resp.AffectedPositions))
	}
	if resp.AffectedPositions[0].PositionID != oldLot.PositionID().String() {
		t.Errorf("expected oldest lot to be consumed first")
	}
}

// Test 8: units_sold computation with rounding.
func TestWithdraw_UnitsSoldRounding(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	// amount=100, unitPrice=7, totalValue=700
	lot := makePosition(clientID, assetID, decimal.NewFromInt(100), decimal.NewFromInt(7), decimal.Zero, decimal.Zero, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 1)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "100" // units_sold = (100/7).Round(6) = 14.285714

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	// new_amount = 100 - 14.285714 = 85.714286
	expectedAmount := "85.714286"
	if resp.AffectedPositions[0].Amount != expectedAmount {
		t.Errorf("expected amount %s, got %s", expectedAmount, resp.AffectedPositions[0].Amount)
	}
}

// --- if_match validation ---

// Test 9: if_match provided and all affected lots match.
func TestWithdraw_IfMatchAllLotsMatch_Proceeds(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	lot := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 5)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "50"
	req.IfMatch = "5" // matches lot.RowVersion()

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// Test 10: if_match provided and one lot has mismatched RowVersion.
func TestWithdraw_IfMatchMismatch_Returns409(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	lot := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 3)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "50"
	req.IfMatch = "1" // does NOT match lot.RowVersion() = 3

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, status)
	}
	var ccErr *ConcurrencyConflictError
	if !errors.As(err, &ccErr) {
		t.Fatalf("expected ConcurrencyConflictError, got %v", err)
	}
	if ccErr.Error() != "concurrency conflict" {
		t.Errorf("expected error message 'concurrency conflict', got %s", ccErr.Error())
	}
}

// Test 11: if_match not provided — validation skipped.
func TestWithdraw_IfMatchNotProvided_Proceeds(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "100"
	// req.IfMatch is empty string → not provided

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// --- Insufficient position ---

// Test 12: Total available value less than desired_value.
func TestWithdraw_InsufficientPosition_Returns409(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	lot := makePosition(clientID, assetID, decimal.NewFromInt(5), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 1)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "999" // available is only 50

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, status)
	}
	var ipErr *InsufficientPositionError
	if !errors.As(err, &ipErr) {
		t.Fatalf("expected InsufficientPositionError, got %v", err)
	}
	if ipErr.Error() != "insufficient position" {
		t.Errorf("expected error message 'insufficient position', got %s", ipErr.Error())
	}
}

// --- No lots found ---

// Test 13: FindByClientAndInstrument returns empty slice.
func TestWithdraw_NoLots_Returns404(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, status)
	}
	if err == nil || err.Error() != "no positions found" {
		t.Fatalf("expected 'no positions found', got %v", err)
	}
}

// --- Idempotency replay ---

// Test 14: Existing ProcessedCommand returns stored snapshot with 200.
func TestWithdraw_IdempotentReplay_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	snapshot := WithdrawResponse{
		AffectedPositions: []AffectedPosition{
			{
				PositionID: uuid.New().String(),
				ClientID:   uuid.New().String(),
				AssetID:    uuid.New().String(),
				Amount:     "50",
				UnitPrice:  "10",
				TotalValue: "500",
			},
		},
	}
	snapshotBytes, _ := json.Marshal(snapshot)

	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		return domain.ReconstructProcessedCommand(
			uuid.New(), "WITHDRAW", "order-w-123", uuid.New(), snapshotBytes, time.Now(),
		), nil
	}

	updateCalled := false
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		updateCalled = true
		return nil
	}
	createCalled := false
	processedCmds.createFn = func(_ context.Context, _ *domain.ProcessedCommand) error {
		createCalled = true
		return nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.OrderID = "order-w-123"

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if resp.AffectedPositions[0].PositionID != snapshot.AffectedPositions[0].PositionID {
		t.Errorf("expected position_id %s, got %s", snapshot.AffectedPositions[0].PositionID, resp.AffectedPositions[0].PositionID)
	}
	if updateCalled {
		t.Error("expected no Update calls for idempotent replay")
	}
	if createCalled {
		t.Error("expected no Create calls for idempotent replay")
	}
}

// --- Idempotency concurrent race ---

// Test 15: ErrDuplicate from Create → re-read returns snapshot with 200.
func TestWithdraw_RaceCondition_ReplayReturns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	snapshot := WithdrawResponse{
		AffectedPositions: []AffectedPosition{
			{
				PositionID: uuid.New().String(),
				Amount:     "50",
			},
		},
	}
	snapshotBytes, _ := json.Marshal(snapshot)

	callCount := 0
	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		callCount++
		if callCount == 1 {
			return nil, domain.ErrNotFound
		}
		return domain.ReconstructProcessedCommand(
			uuid.New(), "WITHDRAW", "order-w-123", uuid.New(), snapshotBytes, time.Now(),
		), nil
	}
	processedCmds.createFn = func(_ context.Context, _ *domain.ProcessedCommand) error {
		return domain.ErrDuplicate
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if resp.AffectedPositions[0].PositionID != snapshot.AffectedPositions[0].PositionID {
		t.Errorf("expected position_id %s, got %s", snapshot.AffectedPositions[0].PositionID, resp.AffectedPositions[0].PositionID)
	}
}

// Test 16: ErrDuplicate from Create → re-read fails → 409.
func TestWithdraw_RaceCondition_RereadFails_Returns409(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	callCount := 0
	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		callCount++
		if callCount == 1 {
			return nil, domain.ErrNotFound
		}
		return nil, errors.New("database gone")
	}
	processedCmds.createFn = func(_ context.Context, _ *domain.ProcessedCommand) error {
		return domain.ErrDuplicate
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if err == nil {
		t.Fatal("expected error")
	}
	if status != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, status)
	}
}

// --- Concurrency conflict from Update ---

// Test 17: PositionRepository.Update returns ErrConcurrencyConflict.
func TestWithdraw_UpdateConcurrencyConflict_Returns409(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return domain.ErrConcurrencyConflict
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "100"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, status)
	}
	var ccErr *ConcurrencyConflictError
	if !errors.As(err, &ccErr) {
		t.Fatalf("expected ConcurrencyConflictError, got %v", err)
	}
}

// --- Validation failure tests (all return 422) ---

// Test 18: Missing client_id.
func TestWithdraw_MissingClientID_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.ClientID = ""

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "client_id is required" {
		t.Fatalf("expected 'client_id is required', got %v", err)
	}
}

// Test 19: Invalid client_id.
func TestWithdraw_InvalidClientID_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.ClientID = "not-a-uuid"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "invalid client_id" {
		t.Fatalf("expected 'invalid client_id', got %v", err)
	}
}

// Test 20: Missing product_asset_id.
func TestWithdraw_MissingProductAssetID_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.ProductAssetID = ""

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "product_asset_id is required" {
		t.Fatalf("expected 'product_asset_id is required', got %v", err)
	}
}

// Test 21: Missing order_id.
func TestWithdraw_MissingOrderID_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.OrderID = ""

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "order_id is required" {
		t.Fatalf("expected 'order_id is required', got %v", err)
	}
}

// Test 22: Missing desired_value.
func TestWithdraw_MissingDesiredValue_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = ""

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "desired_value is required" {
		t.Fatalf("expected 'desired_value is required', got %v", err)
	}
}

// Test 23: Invalid desired_value (not a decimal).
func TestWithdraw_InvalidDesiredValue_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "abc"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "invalid desired_value" {
		t.Fatalf("expected 'invalid desired_value', got %v", err)
	}
}

// Test 24: desired_value is zero.
func TestWithdraw_ZeroDesiredValue_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "0"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "desired_value must be positive" {
		t.Fatalf("expected 'desired_value must be positive', got %v", err)
	}
}

// Test 25: desired_value is negative.
func TestWithdraw_NegativeDesiredValue_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "-10"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "desired_value must be positive" {
		t.Fatalf("expected 'desired_value must be positive', got %v", err)
	}
}

// Test 26: if_match present but not a valid integer.
func TestWithdraw_InvalidIfMatch_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.IfMatch = "not-an-int"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "invalid if_match" {
		t.Fatalf("expected 'invalid if_match', got %v", err)
	}
}

// --- Entity lookup failures ---

// Test 27: Client not found.
func TestWithdraw_ClientNotFound_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clients.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Client, error) {
		return nil, domain.ErrNotFound
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "client not found" {
		t.Fatalf("expected 'client not found', got %v", err)
	}
}

// --- Unexpected error tests (return 500) ---

// Test 28: ClientRepository.FindByID unexpected error.
func TestWithdraw_ClientFindUnexpectedError_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clients.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Client, error) {
		return nil, errors.New("connection refused")
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

// Test 29: ProcessedCommandRepository.FindByTypeAndOrderID unexpected error.
func TestWithdraw_ProcessedCommandFindUnexpectedError_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		return nil, errors.New("database crashed")
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

// Test 30: PositionRepository.FindByClientAndInstrument unexpected error.
func TestWithdraw_PositionFindUnexpectedError_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return nil, errors.New("timeout")
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

// Test 31: UnitOfWork.Do propagates unexpected error from PositionRepository.Update.
func TestWithdraw_UpdateUnexpectedError_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return errors.New("disk full")
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "100"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Additional coverage test for snapshot deserialization error ---

func TestWithdraw_IdempotentReplay_CorruptedSnapshot_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		return domain.ReconstructProcessedCommand(
			uuid.New(), "WITHDRAW", "order-w-123", uuid.New(), []byte("invalid-json"), time.Now(),
		), nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Additional coverage: if_match break when remaining reaches zero ---

func TestWithdraw_IfMatchBreaksWhenRemainingZero(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	clientID := uuid.New()
	assetID := uuid.New()
	// Two eligible lots, both with row_version=2
	lot1 := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 2)
	lot2 := makePosition(clientID, assetID, decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero, decimal.Zero, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), 2)

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot1, lot2}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "100" // Exactly lot1's available value (10*10=100), remaining will be 0 after lot1
	req.IfMatch = "2"

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	// Only lot1 should be affected since its available value covers the full desired_value
	if len(resp.AffectedPositions) != 1 {
		t.Fatalf("expected 1 affected position, got %d", len(resp.AffectedPositions))
	}
}

// --- Additional coverage: nil UUID client_id triggers NewProcessedCommand failure ---

func TestWithdraw_NilUUIDClientID_NewProcessedCommandFails_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.ClientID = uuid.Nil.String()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}
