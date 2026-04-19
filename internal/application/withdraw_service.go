package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/primolabs-org/spec-star-go/internal/ports"
	"github.com/shopspring/decimal"
)

const commandTypeWithdraw = "WITHDRAW"

// InsufficientPositionError indicates the total available value is less than the desired withdrawal.
type InsufficientPositionError struct{}

func (e *InsufficientPositionError) Error() string { return "insufficient position" }

// ConcurrencyConflictError indicates an optimistic concurrency mismatch.
type ConcurrencyConflictError struct{}

func (e *ConcurrencyConflictError) Error() string { return "concurrency conflict" }

// WithdrawRequest holds the raw input fields for a withdraw command.
type WithdrawRequest struct {
	ClientID       string `json:"client_id"`
	ProductAssetID string `json:"product_asset_id"`
	OrderID        string `json:"order_id"`
	DesiredValue   string `json:"desired_value"`
	IfMatch        string `json:"if_match"`
}

// AffectedPosition is the DTO for a position affected by a withdrawal.
type AffectedPosition struct {
	PositionID               string `json:"position_id"`
	ClientID                 string `json:"client_id"`
	AssetID                  string `json:"asset_id"`
	Amount                   string `json:"amount"`
	UnitPrice                string `json:"unit_price"`
	TotalValue               string `json:"total_value"`
	CollateralValue          string `json:"collateral_value"`
	JudiciaryCollateralValue string `json:"judiciary_collateral_value"`
	PurchasedAt              string `json:"purchased_at"`
	CreatedAt                string `json:"created_at"`
	UpdatedAt                string `json:"updated_at"`
}

// WithdrawResponse is the DTO returned to callers after a successful withdrawal.
type WithdrawResponse struct {
	AffectedPositions []AffectedPosition `json:"affected_positions"`
}

// WithdrawService orchestrates the withdrawal command flow.
type WithdrawService struct {
	clients           ports.ClientRepository
	positions         ports.PositionRepository
	processedCommands ports.ProcessedCommandRepository
	unitOfWork        ports.UnitOfWork
}

// NewWithdrawService constructs a WithdrawService with required port dependencies.
func NewWithdrawService(
	clients ports.ClientRepository,
	positions ports.PositionRepository,
	processedCommands ports.ProcessedCommandRepository,
	unitOfWork ports.UnitOfWork,
) *WithdrawService {
	return &WithdrawService{
		clients:           clients,
		positions:         positions,
		processedCommands: processedCommands,
		unitOfWork:        unitOfWork,
	}
}

// Execute validates input, enforces idempotency, selects lots in FIFO order, reduces amounts, and persists atomic state.
func (s *WithdrawService) Execute(ctx context.Context, req WithdrawRequest) (*WithdrawResponse, int, error) {
	// Step 1: Validate input
	clientID, desiredValue, ifMatch, err := validateWithdrawRequest(req)
	if err != nil {
		return nil, http.StatusUnprocessableEntity, err
	}

	// Step 2: Idempotency check
	existing, err := s.processedCommands.FindByTypeAndOrderID(ctx, commandTypeWithdraw, req.OrderID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, http.StatusInternalServerError, fmt.Errorf("find processed command: %w", err)
	}
	if existing != nil {
		return deserializeWithdrawSnapshot(existing.ResponseSnapshot())
	}

	// Step 3: Client lookup
	_, err = s.clients.FindByID(ctx, clientID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, http.StatusUnprocessableEntity, fmt.Errorf("client not found")
		}
		return nil, http.StatusInternalServerError, fmt.Errorf("find client: %w", err)
	}

	// Step 4: Find lots
	lots, err := s.positions.FindByClientAndInstrument(ctx, clientID, req.ProductAssetID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("find positions: %w", err)
	}

	// Step 5: No lots
	if len(lots) == 0 {
		return nil, http.StatusNotFound, fmt.Errorf("no positions found")
	}

	// Step 6: Pass 1 — Sufficiency check (read-only)
	totalAvailable := decimal.Zero
	var eligibleLots []*domain.Position
	for _, lot := range lots {
		av := lot.AvailableValue()
		if av.IsPositive() {
			totalAvailable = totalAvailable.Add(av)
			eligibleLots = append(eligibleLots, lot)
		}
	}

	// Step 7: Insufficient position
	if totalAvailable.LessThan(desiredValue) {
		return nil, http.StatusConflict, &InsufficientPositionError{}
	}

	// Step 8: if_match validation (between pass 1 and pass 2)
	if ifMatch != nil {
		remaining := desiredValue
		for _, lot := range eligibleLots {
			if remaining.IsZero() {
				break
			}
			valueFromLot := decimal.Min(remaining, lot.AvailableValue())
			remaining = remaining.Sub(valueFromLot)
			if lot.RowVersion() != *ifMatch {
				return nil, http.StatusConflict, &ConcurrencyConflictError{}
			}
		}
	}

	// Step 9: Pass 2 — Mutation
	remaining := desiredValue
	var affectedPositions []*domain.Position
	for _, lot := range eligibleLots {
		if remaining.IsZero() {
			break
		}
		valueFromLot := decimal.Min(remaining, lot.AvailableValue())
		unitsSold := valueFromLot.Div(lot.UnitPrice()).Round(6)
		// Pass 1 sufficiency check guarantees non-negative result; UpdateAmount cannot fail here.
		_ = lot.UpdateAmount(lot.Amount().Sub(unitsSold))
		remaining = remaining.Sub(valueFromLot)
		affectedPositions = append(affectedPositions, lot)
	}

	// Step 10: Build response
	resp := &WithdrawResponse{
		AffectedPositions: make([]AffectedPosition, len(affectedPositions)),
	}
	for i, p := range affectedPositions {
		resp.AffectedPositions[i] = toAffectedPosition(p)
	}

	// WithdrawResponse contains only string fields; json.Marshal cannot fail.
	snapshotBytes, _ := json.Marshal(resp)

	// Step 11: Atomic persistence
	err = s.unitOfWork.Do(ctx, func(txCtx context.Context) error {
		for _, p := range affectedPositions {
			if err := s.positions.Update(txCtx, p); err != nil {
				return fmt.Errorf("update position: %w", err)
			}
		}

		cmd, err := domain.NewProcessedCommand(commandTypeWithdraw, req.OrderID, clientID, snapshotBytes)
		if err != nil {
			return fmt.Errorf("new processed command: %w", err)
		}

		if err := s.processedCommands.Create(txCtx, cmd); err != nil {
			return fmt.Errorf("create processed command: %w", err)
		}

		return nil
	})
	if err != nil {
		// Step 12: Handle ErrDuplicate (concurrent race)
		if errors.Is(err, domain.ErrDuplicate) {
			return s.replayAfterRace(ctx, req.OrderID)
		}
		// Step 13: Handle ErrConcurrencyConflict
		if errors.Is(err, domain.ErrConcurrencyConflict) {
			return nil, http.StatusConflict, &ConcurrencyConflictError{}
		}
		return nil, http.StatusInternalServerError, fmt.Errorf("unit of work: %w", err)
	}

	return resp, http.StatusOK, nil
}

func (s *WithdrawService) replayAfterRace(ctx context.Context, orderID string) (*WithdrawResponse, int, error) {
	existing, err := s.processedCommands.FindByTypeAndOrderID(ctx, commandTypeWithdraw, orderID)
	if err != nil {
		return nil, http.StatusConflict, fmt.Errorf("replay after race: %w", err)
	}
	return deserializeWithdrawSnapshot(existing.ResponseSnapshot())
}

func deserializeWithdrawSnapshot(snapshot []byte) (*WithdrawResponse, int, error) {
	var resp WithdrawResponse
	if err := json.Unmarshal(snapshot, &resp); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("unmarshal response snapshot: %w", err)
	}
	return &resp, http.StatusOK, nil
}

func toAffectedPosition(p *domain.Position) AffectedPosition {
	return AffectedPosition{
		PositionID:               p.PositionID().String(),
		ClientID:                 p.ClientID().String(),
		AssetID:                  p.AssetID().String(),
		Amount:                   p.Amount().String(),
		UnitPrice:                p.UnitPrice().String(),
		TotalValue:               p.TotalValue().String(),
		CollateralValue:          p.CollateralValue().String(),
		JudiciaryCollateralValue: p.JudiciaryCollateralValue().String(),
		PurchasedAt:              p.PurchasedAt().Format(time.RFC3339),
		CreatedAt:                p.CreatedAt().Format(time.RFC3339),
		UpdatedAt:                p.UpdatedAt().Format(time.RFC3339),
	}
}

func validateWithdrawRequest(req WithdrawRequest) (uuid.UUID, decimal.Decimal, *int, error) {
	if req.ClientID == "" {
		return uuid.Nil, decimal.Decimal{}, nil, fmt.Errorf("client_id is required")
	}
	clientID, err := uuid.Parse(req.ClientID)
	if err != nil {
		return uuid.Nil, decimal.Decimal{}, nil, fmt.Errorf("invalid client_id")
	}

	if req.ProductAssetID == "" {
		return uuid.Nil, decimal.Decimal{}, nil, fmt.Errorf("product_asset_id is required")
	}

	if req.OrderID == "" {
		return uuid.Nil, decimal.Decimal{}, nil, fmt.Errorf("order_id is required")
	}

	if req.DesiredValue == "" {
		return uuid.Nil, decimal.Decimal{}, nil, fmt.Errorf("desired_value is required")
	}
	desiredValue, err := decimal.NewFromString(req.DesiredValue)
	if err != nil {
		return uuid.Nil, decimal.Decimal{}, nil, fmt.Errorf("invalid desired_value")
	}

	if !desiredValue.IsPositive() {
		return uuid.Nil, decimal.Decimal{}, nil, fmt.Errorf("desired_value must be positive")
	}

	var ifMatch *int
	if req.IfMatch != "" {
		v, err := strconv.Atoi(req.IfMatch)
		if err != nil {
			return uuid.Nil, decimal.Decimal{}, nil, fmt.Errorf("invalid if_match")
		}
		ifMatch = &v
	}

	return clientID, desiredValue, ifMatch, nil
}
