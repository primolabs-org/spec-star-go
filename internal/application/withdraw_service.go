package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/primolabs-org/spec-star-go/internal/ports"
	"github.com/shopspring/decimal"
)

const commandTypeWithdraw = "WITHDRAW"

var withdrawMarshalJSON = json.Marshal

type WithdrawRequest struct {
	ClientID     string `json:"client_id"`
	InstrumentID string `json:"instrument_id"`
	OrderID      string `json:"order_id"`
	DesiredValue string `json:"desired_value"`
}

type WithdrawResponse struct {
	Positions []PositionDTO `json:"positions"`
}

type PositionDTO struct {
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

type WithdrawService struct {
	clients           ports.ClientRepository
	positions         ports.PositionRepository
	processedCommands ports.ProcessedCommandRepository
	unitOfWork        ports.UnitOfWork
}

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

func (s *WithdrawService) Execute(ctx context.Context, req WithdrawRequest) (*WithdrawResponse, int, error) {
	clientID, desiredValue, err := validateWithdrawRequest(req)
	if err != nil {
		return nil, http.StatusUnprocessableEntity, err
	}

	existing, err := s.processedCommands.FindByTypeAndOrderID(ctx, commandTypeWithdraw, req.OrderID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, http.StatusInternalServerError, fmt.Errorf("find processed command: %w", err)
	}
	if existing != nil {
		return deserializeWithdrawSnapshot(existing.ResponseSnapshot())
	}

	_, err = s.clients.FindByID(ctx, clientID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, http.StatusUnprocessableEntity, fmt.Errorf("client not found")
		}
		return nil, http.StatusInternalServerError, fmt.Errorf("find client: %w", err)
	}

	var resp *WithdrawResponse

	err = s.unitOfWork.Do(ctx, func(txCtx context.Context) error {
		positions, err := s.positions.FindByClientAndInstrument(txCtx, clientID, req.InstrumentID)
		if err != nil {
			return fmt.Errorf("find positions: %w", err)
		}
		if len(positions) == 0 {
			return fmt.Errorf("no positions found: %w", domain.ErrNotFound)
		}

		var eligible []*domain.Position
		totalAvailable := decimal.Zero
		for _, p := range positions {
			if p.AvailableValue().IsPositive() {
				eligible = append(eligible, p)
				totalAvailable = totalAvailable.Add(p.AvailableValue())
			}
		}

		if totalAvailable.LessThan(desiredValue) {
			return fmt.Errorf("total available %s less than desired %s: %w", totalAvailable, desiredValue, domain.ErrInsufficientPosition)
		}

		remaining := desiredValue
		var affected []*domain.Position
		for _, lot := range eligible {
			if !remaining.IsPositive() {
				break
			}
			available := lot.AvailableValue()
			valueFromLot := decimal.Min(remaining, available)
			unitsSold := valueFromLot.Div(lot.UnitPrice()).Round(6)
			actualValueConsumed := unitsSold.Mul(lot.UnitPrice())

			newAmount := lot.Amount().Sub(unitsSold)
			if err := lot.UpdateAmount(newAmount); err != nil {
				return fmt.Errorf("update amount: %w", err)
			}

			remaining = remaining.Sub(actualValueConsumed)
			affected = append(affected, lot)
		}

		for _, lot := range affected {
			if err := s.positions.Update(txCtx, lot); err != nil {
				return fmt.Errorf("update position: %w", err)
			}
		}

		resp = toWithdrawResponse(affected)

		snapshotBytes, err := withdrawMarshalJSON(resp)
		if err != nil {
			return fmt.Errorf("marshal response snapshot: %w", err)
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
		if errors.Is(err, domain.ErrDuplicate) {
			return s.replayAfterRace(ctx, req.OrderID)
		}
		if errors.Is(err, domain.ErrConcurrencyConflict) {
			return nil, http.StatusConflict, fmt.Errorf("withdraw: %w", err)
		}
		if errors.Is(err, domain.ErrNotFound) {
			return nil, http.StatusNotFound, fmt.Errorf("withdraw: %w", err)
		}
		if errors.Is(err, domain.ErrInsufficientPosition) {
			return nil, http.StatusConflict, fmt.Errorf("withdraw: %w", err)
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

func toWithdrawResponse(positions []*domain.Position) *WithdrawResponse {
	dtos := make([]PositionDTO, len(positions))
	for i, p := range positions {
		dtos[i] = PositionDTO{
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
	return &WithdrawResponse{Positions: dtos}
}

func validateWithdrawRequest(req WithdrawRequest) (uuid.UUID, decimal.Decimal, error) {
	if req.ClientID == "" {
		return uuid.Nil, decimal.Decimal{}, fmt.Errorf("client_id is required")
	}
	clientID, err := uuid.Parse(req.ClientID)
	if err != nil {
		return uuid.Nil, decimal.Decimal{}, fmt.Errorf("invalid client_id")
	}

	if req.InstrumentID == "" {
		return uuid.Nil, decimal.Decimal{}, fmt.Errorf("instrument_id is required")
	}

	if req.OrderID == "" {
		return uuid.Nil, decimal.Decimal{}, fmt.Errorf("order_id is required")
	}

	if req.DesiredValue == "" {
		return uuid.Nil, decimal.Decimal{}, fmt.Errorf("desired_value is required")
	}
	desiredValue, err := decimal.NewFromString(req.DesiredValue)
	if err != nil {
		return uuid.Nil, decimal.Decimal{}, fmt.Errorf("invalid desired_value")
	}
	if !desiredValue.IsPositive() {
		return uuid.Nil, decimal.Decimal{}, fmt.Errorf("desired_value must be positive")
	}

	return clientID, desiredValue, nil
}
