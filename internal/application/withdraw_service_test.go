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

type withdrawMockPositionRepository struct {
	findByClientAndInstrumentFn func(ctx context.Context, clientID uuid.UUID, instrumentID string) ([]*domain.Position, error)
	updateFn                    func(ctx context.Context, position *domain.Position) error
}

func (m *withdrawMockPositionRepository) FindByID(_ context.Context, _ uuid.UUID) (*domain.Position, error) {
	return nil, nil
}

func (m *withdrawMockPositionRepository) FindByClientAndAsset(_ context.Context, _, _ uuid.UUID) ([]*domain.Position, error) {
	return nil, nil
}

func (m *withdrawMockPositionRepository) FindByClientAndInstrument(ctx context.Context, clientID uuid.UUID, instrumentID string) ([]*domain.Position, error) {
	return m.findByClientAndInstrumentFn(ctx, clientID, instrumentID)
}

func (m *withdrawMockPositionRepository) Create(_ context.Context, _ *domain.Position) error {
	return nil
}

func (m *withdrawMockPositionRepository) Update(ctx context.Context, position *domain.Position) error {
	return m.updateFn(ctx, position)
}

// --- Withdraw test helpers ---

func validWithdrawRequest() WithdrawRequest {
	return WithdrawRequest{
		ClientID:     uuid.New().String(),
		InstrumentID: "INST-001",
		OrderID:      "order-w-123",
		DesiredValue: "500",
	}
}

func buildWithdrawService(
	clients *mockClientRepository,
	positions *withdrawMockPositionRepository,
	processedCmds *mockProcessedCommandRepository,
	uow *mockUnitOfWork,
) *WithdrawService {
	return NewWithdrawService(clients, positions, processedCmds, uow)
}

func withdrawDefaultMocks() (*mockClientRepository, *withdrawMockPositionRepository, *mockProcessedCommandRepository, *mockUnitOfWork) {
	clients := &mockClientRepository{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Client, error) {
			return newTestClient(), nil
		},
	}
	positions := &withdrawMockPositionRepository{
		findByClientAndInstrumentFn: func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
			return nil, nil
		},
		updateFn: func(_ context.Context, _ *domain.Position) error {
			return nil
		},
	}
	processedCmds := &mockProcessedCommandRepository{
		findByTypeAndOrderIDFn: func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
			return nil, domain.ErrNotFound
		},
		createFn: func(_ context.Context, _ *domain.ProcessedCommand) error {
			return nil
		},
	}
	uow := &mockUnitOfWork{
		doFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	}
	return clients, positions, processedCmds, uow
}

func makePosition(t *testing.T, clientID, assetID uuid.UUID, amount, unitPrice string, purchasedAt time.Time) *domain.Position {
	t.Helper()
	amt, err := decimal.NewFromString(amount)
	if err != nil {
		t.Fatalf("invalid amount %q: %v", amount, err)
	}
	up, err := decimal.NewFromString(unitPrice)
	if err != nil {
		t.Fatalf("invalid unitPrice %q: %v", unitPrice, err)
	}
	totalValue := amt.Mul(up)
	p, err := domain.ReconstructPosition(
		uuid.New(), clientID, assetID, amt, up, totalValue,
		decimal.Zero, decimal.Zero,
		purchasedAt, purchasedAt, purchasedAt, 1,
	)
	if err != nil {
		t.Fatalf("reconstruct position: %v", err)
	}
	return p
}

// --- Test 1: Successful single-lot withdrawal ---

func TestWithdrawExecute_SingleLot_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "500"

	lot := makePosition(t, clientID, assetID, "100", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}

	var updatedPositions []*domain.Position
	positions.updateFn = func(_ context.Context, p *domain.Position) error {
		updatedPositions = append(updatedPositions, p)
		return nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

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
	if len(resp.Positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(resp.Positions))
	}
	if len(updatedPositions) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(updatedPositions))
	}

	respAmount, _ := decimal.NewFromString(resp.Positions[0].Amount)
	originalAmount, _ := decimal.NewFromString("100")
	if respAmount.GreaterThanOrEqual(originalAmount) {
		t.Errorf("expected amount to be reduced, got %s", resp.Positions[0].Amount)
	}
}

// --- Test 2: Successful multi-lot FIFO withdrawal ---

func TestWithdrawExecute_MultiLotFIFO_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "150"

	lot1 := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	lot2 := makePosition(t, clientID, assetID, "20", "10", time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot1, lot2}, nil
	}

	var updateOrder []uuid.UUID
	positions.updateFn = func(_ context.Context, p *domain.Position) error {
		updateOrder = append(updateOrder, p.PositionID())
		return nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if len(resp.Positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(resp.Positions))
	}
	if len(updateOrder) != 2 {
		t.Fatalf("expected 2 update calls, got %d", len(updateOrder))
	}
	if updateOrder[0] != lot1.PositionID() {
		t.Errorf("expected first update for lot1, got %s", updateOrder[0])
	}
	if updateOrder[1] != lot2.PositionID() {
		t.Errorf("expected second update for lot2, got %s", updateOrder[1])
	}
}

// --- Test 3: Desired value equals total available ---

func TestWithdrawExecute_DesiredEqualsTotal_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "300"

	lot1 := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	lot2 := makePosition(t, clientID, assetID, "20", "10", time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot1, lot2}, nil
	}
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if len(resp.Positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(resp.Positions))
	}

	for _, p := range resp.Positions {
		amount, _ := decimal.NewFromString(p.Amount)
		if !amount.IsZero() {
			t.Errorf("expected amount 0 for fully consumed lot, got %s", p.Amount)
		}
	}
}

// --- Test 4: Lot with zero available value skipped ---

func TestWithdrawExecute_ZeroAvailableLotSkipped_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "50"

	zeroLot, err := domain.ReconstructPosition(
		uuid.New(), clientID, assetID,
		decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.NewFromInt(100),
		decimal.NewFromInt(100), decimal.Zero,
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		1,
	)
	if err != nil {
		t.Fatalf("reconstruct position: %v", err)
	}

	eligibleLot := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{zeroLot, eligibleLot}, nil
	}

	var updatedIDs []uuid.UUID
	positions.updateFn = func(_ context.Context, p *domain.Position) error {
		updatedIDs = append(updatedIDs, p.PositionID())
		return nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if len(resp.Positions) != 1 {
		t.Fatalf("expected 1 position in response, got %d", len(resp.Positions))
	}
	if resp.Positions[0].PositionID != eligibleLot.PositionID().String() {
		t.Errorf("expected eligible lot in response, got %s", resp.Positions[0].PositionID)
	}
	if len(updatedIDs) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(updatedIDs))
	}
	if updatedIDs[0] != eligibleLot.PositionID() {
		t.Errorf("expected update for eligible lot, got %s", updatedIDs[0])
	}
}

// --- Test 5: INSUFFICIENT_POSITION rejection ---

func TestWithdrawExecute_InsufficientPosition_Returns409(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "5000"

	lot := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrInsufficientPosition) {
		t.Errorf("expected ErrInsufficientPosition, got %v", err)
	}
}

// --- Test 6: No positions found ---

func TestWithdrawExecute_NoPositions_Returns404(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	req := validWithdrawRequest()

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{}, nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Test 7: Concurrency conflict from Update ---

func TestWithdrawExecute_ConcurrencyConflict_Returns409(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "50"

	lot := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return domain.ErrConcurrencyConflict
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrConcurrencyConflict) {
		t.Errorf("expected ErrConcurrencyConflict, got %v", err)
	}
}

// --- Test 8: Idempotent replay ---

func TestWithdrawExecute_IdempotentReplay_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	snapshot := WithdrawResponse{
		Positions: []PositionDTO{
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
			uuid.New(), commandTypeWithdraw, "order-w-123", uuid.New(), snapshotBytes, time.Now(),
		), nil
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
	if len(resp.Positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(resp.Positions))
	}
	if resp.Positions[0].PositionID != snapshot.Positions[0].PositionID {
		t.Errorf("expected position_id %s, got %s", snapshot.Positions[0].PositionID, resp.Positions[0].PositionID)
	}
}

// --- Test 9: Race replay (success) ---

func TestWithdrawExecute_RaceReplay_Success_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "50"

	lot := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return nil
	}

	snapshot := WithdrawResponse{
		Positions: []PositionDTO{
			{
				PositionID: uuid.New().String(),
				ClientID:   clientID.String(),
				AssetID:    assetID.String(),
				Amount:     "5",
				UnitPrice:  "10",
				TotalValue: "50",
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
			uuid.New(), commandTypeWithdraw, req.OrderID, clientID, snapshotBytes, time.Now(),
		), nil
	}

	processedCmds.createFn = func(_ context.Context, _ *domain.ProcessedCommand) error {
		return domain.ErrDuplicate
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if resp.Positions[0].PositionID != snapshot.Positions[0].PositionID {
		t.Errorf("expected position_id %s, got %s", snapshot.Positions[0].PositionID, resp.Positions[0].PositionID)
	}
}

// --- Test 10: Race replay (re-read fails) ---

func TestWithdrawExecute_RaceReplay_RereadFails_Returns409(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "50"

	lot := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return nil
	}

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

	_, status, err := svc.Execute(context.Background(), req)

	if err == nil {
		t.Fatal("expected error")
	}
	if status != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, status)
	}
}

// --- Test 11: Validation: client_id missing ---

func TestWithdrawExecute_MissingClientID_Returns422(t *testing.T) {
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

// --- Test 12: Validation: client_id invalid UUID ---

func TestWithdrawExecute_InvalidClientID_Returns422(t *testing.T) {
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

// --- Test 13: Validation: instrument_id missing ---

func TestWithdrawExecute_MissingInstrumentID_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.InstrumentID = ""

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "instrument_id is required" {
		t.Fatalf("expected 'instrument_id is required', got %v", err)
	}
}

// --- Test 14: Validation: order_id missing ---

func TestWithdrawExecute_MissingOrderID_Returns422(t *testing.T) {
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

// --- Test 15: Validation: desired_value missing ---

func TestWithdrawExecute_MissingDesiredValue_Returns422(t *testing.T) {
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

// --- Test 16: Validation: desired_value invalid decimal ---

func TestWithdrawExecute_InvalidDesiredValue_Returns422(t *testing.T) {
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

// --- Test 17: Validation: desired_value zero or negative ---

func TestWithdrawExecute_ZeroDesiredValue_Returns422(t *testing.T) {
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

func TestWithdrawExecute_NegativeDesiredValue_Returns422(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	svc := buildWithdrawService(clients, positions, processedCmds, uow)
	req := validWithdrawRequest()
	req.DesiredValue = "-100"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "desired_value must be positive" {
		t.Fatalf("expected 'desired_value must be positive', got %v", err)
	}
}

// --- Test 18: Validation: client not found ---

func TestWithdrawExecute_ClientNotFound_Returns422(t *testing.T) {
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

// --- Test 19: Rounding verification ---

func TestWithdrawExecute_RoundingVerification_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "100"

	lot := makePosition(t, clientID, assetID, "100", "3", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	respAmount, _ := decimal.NewFromString(resp.Positions[0].Amount)
	unitPrice := decimal.NewFromInt(3)

	unitsSold := decimal.NewFromInt(100).Div(unitPrice).Round(6)
	expectedUnitsSold, _ := decimal.NewFromString("33.333333")
	if !unitsSold.Equal(expectedUnitsSold) {
		t.Errorf("expected units_sold %s, got %s", expectedUnitsSold, unitsSold)
	}

	actualConsumed := unitsSold.Mul(unitPrice)
	expectedConsumed, _ := decimal.NewFromString("99.999999")
	if !actualConsumed.Equal(expectedConsumed) {
		t.Errorf("expected actual_consumed %s, got %s", expectedConsumed, actualConsumed)
	}

	expectedRemaining := decimal.NewFromInt(100).Sub(unitsSold)
	if !respAmount.Equal(expectedRemaining) {
		t.Errorf("expected remaining amount %s, got %s", expectedRemaining, respAmount)
	}
}

// --- Additional coverage tests ---

func TestWithdrawExecute_FindProcessedCommandError_Returns500(t *testing.T) {
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

func TestWithdrawExecute_ClientFindUnexpectedError_Returns500(t *testing.T) {
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

func TestWithdrawExecute_FindPositionsError_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return nil, errors.New("db timeout")
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

func TestWithdrawExecute_FIFOBreakExtraLot_Returns200(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "50"

	lot1 := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	lot2 := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC))
	lot3 := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot1, lot2, lot3}, nil
	}

	var updatedIDs []uuid.UUID
	positions.updateFn = func(_ context.Context, p *domain.Position) error {
		updatedIDs = append(updatedIDs, p.PositionID())
		return nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if len(resp.Positions) != 1 {
		t.Fatalf("expected 1 position (only lot1 needed for 50), got %d", len(resp.Positions))
	}
	if len(updatedIDs) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(updatedIDs))
	}
}

func TestWithdrawExecute_NilClientID_NewProcessedCommandFails_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.Nil
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "50"

	lot := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWithdrawExecute_CorruptSnapshot_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()

	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		return domain.ReconstructProcessedCommand(
			uuid.New(), commandTypeWithdraw, "order-w-123", uuid.New(), []byte("not-json"), time.Now(),
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

func TestWithdrawExecute_PositionUpdateError_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "50"

	lot := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return errors.New("disk full")
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWithdrawExecute_MarshalSnapshotError_Returns500(t *testing.T) {
	original := withdrawMarshalJSON
	withdrawMarshalJSON = func(v any) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}
	t.Cleanup(func() { withdrawMarshalJSON = original })

	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "50"

	lot := makePosition(t, clientID, assetID, "10", "10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWithdrawExecute_UpdateAmountError_RoundingCausesNegative_Returns500(t *testing.T) {
	clients, positions, processedCmds, uow := withdrawDefaultMocks()
	clientID := uuid.New()
	assetID := uuid.New()
	req := validWithdrawRequest()
	req.ClientID = clientID.String()
	req.DesiredValue = "0.0000005"

	amt, _ := decimal.NewFromString("0.0000005")
	up := decimal.NewFromInt(1)
	tv := amt.Mul(up)
	lot, err := domain.ReconstructPosition(
		uuid.New(), clientID, assetID,
		amt, up, tv,
		decimal.Zero, decimal.Zero,
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		1,
	)
	if err != nil {
		t.Fatalf("reconstruct position: %v", err)
	}

	positions.findByClientAndInstrumentFn = func(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
		return []*domain.Position{lot}, nil
	}
	positions.updateFn = func(_ context.Context, _ *domain.Position) error {
		return nil
	}

	svc := buildWithdrawService(clients, positions, processedCmds, uow)

	_, status, err2 := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err2 == nil {
		t.Fatal("expected error")
	}
}
