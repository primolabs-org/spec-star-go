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

const commandTypeDeposit = "DEPOSIT"

// DepositRequest holds the raw input fields for a deposit command.
type DepositRequest struct {
	ClientID  string `json:"client_id"`
	AssetID   string `json:"asset_id"`
	OrderID   string `json:"order_id"`
	Amount    string `json:"amount"`
	UnitPrice string `json:"unit_price"`
}

// DepositResponse is the DTO returned to callers after a successful deposit.
type DepositResponse struct {
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

// DepositService orchestrates the deposit command flow.
type DepositService struct {
	clients           ports.ClientRepository
	assets            ports.AssetRepository
	positions         ports.PositionRepository
	processedCommands ports.ProcessedCommandRepository
	unitOfWork        ports.UnitOfWork
}

// NewDepositService constructs a DepositService with required port dependencies.
func NewDepositService(
	clients ports.ClientRepository,
	assets ports.AssetRepository,
	positions ports.PositionRepository,
	processedCommands ports.ProcessedCommandRepository,
	unitOfWork ports.UnitOfWork,
) *DepositService {
	return &DepositService{
		clients:           clients,
		assets:            assets,
		positions:         positions,
		processedCommands: processedCommands,
		unitOfWork:        unitOfWork,
	}
}

// Execute validates input, enforces idempotency, creates a position, and persists atomic state.
func (s *DepositService) Execute(ctx context.Context, req DepositRequest) (*DepositResponse, int, error) {
	clientID, assetID, amount, unitPrice, err := validateDepositRequest(req)
	if err != nil {
		return nil, http.StatusUnprocessableEntity, err
	}

	existing, err := s.processedCommands.FindByTypeAndOrderID(ctx, commandTypeDeposit, req.OrderID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, http.StatusInternalServerError, fmt.Errorf("find processed command: %w", err)
	}
	if existing != nil {
		return deserializeSnapshot(existing.ResponseSnapshot())
	}

	_, err = s.clients.FindByID(ctx, clientID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, http.StatusUnprocessableEntity, fmt.Errorf("client not found")
		}
		return nil, http.StatusInternalServerError, fmt.Errorf("find client: %w", err)
	}

	asset, err := s.assets.FindByID(ctx, assetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, http.StatusUnprocessableEntity, fmt.Errorf("asset not found")
		}
		return nil, http.StatusInternalServerError, fmt.Errorf("find asset: %w", err)
	}

	if err := domain.ValidateProductType(asset.ProductType()); err != nil {
		return nil, http.StatusUnprocessableEntity, fmt.Errorf("unsupported product type")
	}

	position, err := domain.NewPosition(clientID, assetID, amount, unitPrice, time.Now().UTC())
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("new position: %w", err)
	}

	resp := toDepositResponse(position)

	snapshotBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("marshal response snapshot: %w", err)
	}

	err = s.unitOfWork.Do(ctx, func(txCtx context.Context) error {
		if err := s.positions.Create(txCtx, position); err != nil {
			return fmt.Errorf("create position: %w", err)
		}

		cmd, err := domain.NewProcessedCommand(commandTypeDeposit, req.OrderID, clientID, snapshotBytes)
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
		return nil, http.StatusInternalServerError, fmt.Errorf("unit of work: %w", err)
	}

	return resp, http.StatusCreated, nil
}

func (s *DepositService) replayAfterRace(ctx context.Context, orderID string) (*DepositResponse, int, error) {
	existing, err := s.processedCommands.FindByTypeAndOrderID(ctx, commandTypeDeposit, orderID)
	if err != nil {
		return nil, http.StatusConflict, fmt.Errorf("replay after race: %w", err)
	}
	return deserializeSnapshot(existing.ResponseSnapshot())
}

func deserializeSnapshot(snapshot []byte) (*DepositResponse, int, error) {
	var resp DepositResponse
	if err := json.Unmarshal(snapshot, &resp); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("unmarshal response snapshot: %w", err)
	}
	return &resp, http.StatusOK, nil
}

func toDepositResponse(p *domain.Position) *DepositResponse {
	return &DepositResponse{
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

func validateDepositRequest(req DepositRequest) (uuid.UUID, uuid.UUID, decimal.Decimal, decimal.Decimal, error) {
	if req.ClientID == "" {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("client_id is required")
	}
	clientID, err := uuid.Parse(req.ClientID)
	if err != nil {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("invalid client_id")
	}

	if req.AssetID == "" {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("asset_id is required")
	}
	assetID, err := uuid.Parse(req.AssetID)
	if err != nil {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("invalid asset_id")
	}

	if req.OrderID == "" {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("order_id is required")
	}

	if req.Amount == "" {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("amount is required")
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("invalid amount")
	}

	if req.UnitPrice == "" {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("unit_price is required")
	}
	unitPrice, err := decimal.NewFromString(req.UnitPrice)
	if err != nil {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("invalid unit_price")
	}

	if !amount.IsPositive() {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("amount must be positive")
	}
	if !unitPrice.IsPositive() {
		return uuid.Nil, uuid.Nil, decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("unit_price must be positive")
	}

	return clientID, assetID, amount, unitPrice, nil
}
