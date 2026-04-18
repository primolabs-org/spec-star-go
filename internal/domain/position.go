package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Position represents a single deposit lot for a client's holding of an asset.
type Position struct {
	positionID               uuid.UUID
	clientID                 uuid.UUID
	assetID                  uuid.UUID
	amount                   decimal.Decimal
	unitPrice                decimal.Decimal
	totalValue               decimal.Decimal
	collateralValue          decimal.Decimal
	judiciaryCollateralValue decimal.Decimal
	createdAt                time.Time
	updatedAt                time.Time
	purchasedAt              time.Time
	rowVersion               int
}

// NewPosition creates a Position with a generated UUID, derived totalValue, and initial concurrency token.
func NewPosition(
	clientID uuid.UUID,
	assetID uuid.UUID,
	amount decimal.Decimal,
	unitPrice decimal.Decimal,
	purchasedAt time.Time,
) (*Position, error) {
	if err := validatePositionAmounts(amount, unitPrice); err != nil {
		return nil, err
	}
	now := time.Now()
	return &Position{
		positionID:               uuid.New(),
		clientID:                 clientID,
		assetID:                  assetID,
		amount:                   amount,
		unitPrice:                unitPrice,
		totalValue:               amount.Mul(unitPrice),
		collateralValue:          decimal.Zero,
		judiciaryCollateralValue: decimal.Zero,
		createdAt:                now,
		updatedAt:                now,
		purchasedAt:              purchasedAt,
		rowVersion:               1,
	}, nil
}

func validatePositionAmounts(amount, unitPrice decimal.Decimal) error {
	if amount.IsNegative() {
		return &ValidationError{Message: "amount must be non-negative"}
	}
	if unitPrice.IsNegative() {
		return &ValidationError{Message: "unit_price must be non-negative"}
	}
	return nil
}

// ReconstructPosition loads a Position from persisted fields and validates structural integrity.
func ReconstructPosition(
	positionID uuid.UUID,
	clientID uuid.UUID,
	assetID uuid.UUID,
	amount decimal.Decimal,
	unitPrice decimal.Decimal,
	totalValue decimal.Decimal,
	collateralValue decimal.Decimal,
	judiciaryCollateralValue decimal.Decimal,
	createdAt time.Time,
	updatedAt time.Time,
	purchasedAt time.Time,
	rowVersion int,
) (*Position, error) {
	if err := validatePositionAmounts(amount, unitPrice); err != nil {
		return nil, err
	}
	if collateralValue.IsNegative() {
		return nil, &ValidationError{Message: "collateral_value must be non-negative"}
	}
	if judiciaryCollateralValue.IsNegative() {
		return nil, &ValidationError{Message: "judiciary_collateral_value must be non-negative"}
	}
	if !totalValue.Equal(amount.Mul(unitPrice)) {
		return nil, &ValidationError{Message: "total_value must equal amount × unit_price"}
	}
	return &Position{
		positionID:               positionID,
		clientID:                 clientID,
		assetID:                  assetID,
		amount:                   amount,
		unitPrice:                unitPrice,
		totalValue:               totalValue,
		collateralValue:          collateralValue,
		judiciaryCollateralValue: judiciaryCollateralValue,
		createdAt:                createdAt,
		updatedAt:                updatedAt,
		purchasedAt:              purchasedAt,
		rowVersion:               rowVersion,
	}, nil
}

// AvailableValue returns totalValue minus blocked collateral components.
func (p *Position) AvailableValue() decimal.Decimal {
	return p.totalValue.Sub(p.collateralValue).Sub(p.judiciaryCollateralValue)
}

// UpdateAmount sets a new amount, re-derives totalValue, increments rowVersion, and updates the timestamp.
func (p *Position) UpdateAmount(newAmount decimal.Decimal) error {
	if newAmount.IsNegative() {
		return &ValidationError{Message: "amount must be non-negative"}
	}
	p.amount = newAmount
	p.totalValue = p.amount.Mul(p.unitPrice)
	p.rowVersion++
	p.updatedAt = time.Now()
	return nil
}

// UpdateCollateral sets a new collateral value, increments rowVersion, and updates the timestamp.
func (p *Position) UpdateCollateral(collateral decimal.Decimal) error {
	if collateral.IsNegative() {
		return &ValidationError{Message: "collateral_value must be non-negative"}
	}
	p.collateralValue = collateral
	p.rowVersion++
	p.updatedAt = time.Now()
	return nil
}

// UpdateJudiciaryCollateral sets a new judiciary collateral value, increments rowVersion, and updates the timestamp.
func (p *Position) UpdateJudiciaryCollateral(judiciaryCollateral decimal.Decimal) error {
	if judiciaryCollateral.IsNegative() {
		return &ValidationError{Message: "judiciary_collateral_value must be non-negative"}
	}
	p.judiciaryCollateralValue = judiciaryCollateral
	p.rowVersion++
	p.updatedAt = time.Now()
	return nil
}

func (p *Position) PositionID() uuid.UUID                     { return p.positionID }
func (p *Position) ClientID() uuid.UUID                       { return p.clientID }
func (p *Position) AssetID() uuid.UUID                        { return p.assetID }
func (p *Position) Amount() decimal.Decimal                   { return p.amount }
func (p *Position) UnitPrice() decimal.Decimal                { return p.unitPrice }
func (p *Position) TotalValue() decimal.Decimal               { return p.totalValue }
func (p *Position) CollateralValue() decimal.Decimal          { return p.collateralValue }
func (p *Position) JudiciaryCollateralValue() decimal.Decimal { return p.judiciaryCollateralValue }
func (p *Position) CreatedAt() time.Time                      { return p.createdAt }
func (p *Position) UpdatedAt() time.Time                      { return p.updatedAt }
func (p *Position) PurchasedAt() time.Time                    { return p.purchasedAt }
func (p *Position) RowVersion() int                           { return p.rowVersion }
