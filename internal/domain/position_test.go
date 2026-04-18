package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/shopspring/decimal"
)

func TestNewPosition_ValidCreation(t *testing.T) {
	clientID := uuid.New()
	assetID := uuid.New()
	amount := decimal.NewFromInt(100)
	unitPrice := decimal.RequireFromString("10.50")
	purchasedAt := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	before := time.Now()
	p, err := domain.NewPosition(clientID, assetID, amount, unitPrice, purchasedAt)
	after := time.Now()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.PositionID() == uuid.Nil {
		t.Fatal("expected non-nil position_id")
	}
	if p.ClientID() != clientID {
		t.Fatalf("expected client_id %v, got %v", clientID, p.ClientID())
	}
	if p.AssetID() != assetID {
		t.Fatalf("expected asset_id %v, got %v", assetID, p.AssetID())
	}
	expectedTotal := amount.Mul(unitPrice)
	if !p.TotalValue().Equal(expectedTotal) {
		t.Fatalf("expected total_value %s, got %s", expectedTotal, p.TotalValue())
	}
	if !p.CollateralValue().Equal(decimal.Zero) {
		t.Fatalf("expected collateral_value 0, got %s", p.CollateralValue())
	}
	if !p.JudiciaryCollateralValue().Equal(decimal.Zero) {
		t.Fatalf("expected judiciary_collateral_value 0, got %s", p.JudiciaryCollateralValue())
	}
	if p.RowVersion() != 1 {
		t.Fatalf("expected row_version 1, got %d", p.RowVersion())
	}
	if p.CreatedAt().Before(before) || p.CreatedAt().After(after) {
		t.Fatal("expected created_at within bounds")
	}
	if !p.CreatedAt().Equal(p.UpdatedAt()) {
		t.Fatal("expected created_at == updated_at on creation")
	}
	if !p.PurchasedAt().Equal(purchasedAt) {
		t.Fatalf("expected purchased_at %v, got %v", purchasedAt, p.PurchasedAt())
	}
}

func TestNewPosition_ZeroAmount(t *testing.T) {
	p, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.Zero, decimal.NewFromInt(10), time.Now())
	if err != nil {
		t.Fatalf("expected zero amount to be accepted, got error: %v", err)
	}
	if !p.TotalValue().Equal(decimal.Zero) {
		t.Fatalf("expected total_value 0, got %s", p.TotalValue())
	}
}

func TestNewPosition_NegativeAmount(t *testing.T) {
	_, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(-1), decimal.NewFromInt(10), time.Now())
	if err == nil {
		t.Fatal("expected error for negative amount")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewPosition_NegativeUnitPrice(t *testing.T) {
	_, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(10), decimal.NewFromInt(-1), time.Now())
	if err == nil {
		t.Fatal("expected error for negative unit_price")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestPosition_AvailableValue(t *testing.T) {
	amount := decimal.NewFromInt(100)
	unitPrice := decimal.NewFromInt(10)
	totalValue := amount.Mul(unitPrice)
	collateral := decimal.NewFromInt(200)
	judiciaryCollateral := decimal.NewFromInt(100)

	p, err := domain.ReconstructPosition(
		uuid.New(), uuid.New(), uuid.New(),
		amount, unitPrice, totalValue,
		collateral, judiciaryCollateral,
		time.Now(), time.Now(), time.Now(), 1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := totalValue.Sub(collateral).Sub(judiciaryCollateral)
	if !p.AvailableValue().Equal(expected) {
		t.Fatalf("expected available_value %s, got %s", expected, p.AvailableValue())
	}
}

func TestPosition_AvailableValue_ZeroCollaterals(t *testing.T) {
	p, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(50), decimal.NewFromInt(20), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.AvailableValue().Equal(p.TotalValue()) {
		t.Fatalf("expected available_value %s to equal total_value %s", p.AvailableValue(), p.TotalValue())
	}
}

func TestPosition_UpdateAmount(t *testing.T) {
	p, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(100), decimal.NewFromInt(10), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oldUpdatedAt := p.UpdatedAt()

	if err := p.UpdateAmount(decimal.NewFromInt(200)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedTotal := decimal.NewFromInt(200).Mul(decimal.NewFromInt(10))
	if !p.TotalValue().Equal(expectedTotal) {
		t.Fatalf("expected total_value %s, got %s", expectedTotal, p.TotalValue())
	}
	if !p.Amount().Equal(decimal.NewFromInt(200)) {
		t.Fatalf("expected amount 200, got %s", p.Amount())
	}
	if p.RowVersion() != 2 {
		t.Fatalf("expected row_version 2, got %d", p.RowVersion())
	}
	if p.UpdatedAt().Before(oldUpdatedAt) {
		t.Fatal("expected updated_at to be >= old updated_at")
	}
}

func TestPosition_UpdateAmount_Negative(t *testing.T) {
	p, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(100), decimal.NewFromInt(10), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.UpdateAmount(decimal.NewFromInt(-1))
	if err == nil {
		t.Fatal("expected error for negative amount")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if p.RowVersion() != 1 {
		t.Fatalf("expected row_version unchanged at 1, got %d", p.RowVersion())
	}
}

func TestPosition_UpdateCollateral_Valid(t *testing.T) {
	p, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(100), decimal.NewFromInt(10), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.UpdateCollateral(decimal.NewFromInt(500)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.CollateralValue().Equal(decimal.NewFromInt(500)) {
		t.Fatalf("expected collateral_value 500, got %s", p.CollateralValue())
	}
	if p.RowVersion() != 2 {
		t.Fatalf("expected row_version 2, got %d", p.RowVersion())
	}
}

func TestPosition_UpdateCollateral_Negative(t *testing.T) {
	p, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(100), decimal.NewFromInt(10), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.UpdateCollateral(decimal.NewFromInt(-1))
	if err == nil {
		t.Fatal("expected error for negative collateral")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestPosition_UpdateJudiciaryCollateral_Valid(t *testing.T) {
	p, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(100), decimal.NewFromInt(10), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.UpdateJudiciaryCollateral(decimal.NewFromInt(300)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.JudiciaryCollateralValue().Equal(decimal.NewFromInt(300)) {
		t.Fatalf("expected judiciary_collateral_value 300, got %s", p.JudiciaryCollateralValue())
	}
	if p.RowVersion() != 2 {
		t.Fatalf("expected row_version 2, got %d", p.RowVersion())
	}
}

func TestPosition_UpdateJudiciaryCollateral_Negative(t *testing.T) {
	p, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(100), decimal.NewFromInt(10), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.UpdateJudiciaryCollateral(decimal.NewFromInt(-1))
	if err == nil {
		t.Fatal("expected error for negative judiciary collateral")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestPosition_MultipleMutationsIncrementRowVersion(t *testing.T) {
	p, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(100), decimal.NewFromInt(10), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.UpdateAmount(decimal.NewFromInt(200)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.UpdateCollateral(decimal.NewFromInt(50)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.UpdateJudiciaryCollateral(decimal.NewFromInt(25)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.RowVersion() != 4 {
		t.Fatalf("expected row_version 4 after 3 mutations, got %d", p.RowVersion())
	}
}

func TestPosition_RowVersionStartsAtOne(t *testing.T) {
	p, err := domain.NewPosition(uuid.New(), uuid.New(), decimal.NewFromInt(10), decimal.NewFromInt(1), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.RowVersion() != 1 {
		t.Fatalf("expected row_version 1, got %d", p.RowVersion())
	}
}

func TestReconstructPosition_ValidFields(t *testing.T) {
	posID := uuid.New()
	clientID := uuid.New()
	assetID := uuid.New()
	amount := decimal.NewFromInt(50)
	unitPrice := decimal.RequireFromString("20.5")
	totalValue := amount.Mul(unitPrice)
	collateral := decimal.NewFromInt(100)
	judiciary := decimal.NewFromInt(50)
	created := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	purchased := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	p, err := domain.ReconstructPosition(posID, clientID, assetID, amount, unitPrice, totalValue, collateral, judiciary, created, updated, purchased, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.PositionID() != posID {
		t.Fatalf("expected position_id %v, got %v", posID, p.PositionID())
	}
	if p.ClientID() != clientID {
		t.Fatalf("expected client_id %v, got %v", clientID, p.ClientID())
	}
	if p.AssetID() != assetID {
		t.Fatalf("expected asset_id %v, got %v", assetID, p.AssetID())
	}
	if !p.Amount().Equal(amount) {
		t.Fatalf("expected amount %s, got %s", amount, p.Amount())
	}
	if !p.UnitPrice().Equal(unitPrice) {
		t.Fatalf("expected unit_price %s, got %s", unitPrice, p.UnitPrice())
	}
	if !p.TotalValue().Equal(totalValue) {
		t.Fatalf("expected total_value %s, got %s", totalValue, p.TotalValue())
	}
	if !p.CollateralValue().Equal(collateral) {
		t.Fatalf("expected collateral_value %s, got %s", collateral, p.CollateralValue())
	}
	if !p.JudiciaryCollateralValue().Equal(judiciary) {
		t.Fatalf("expected judiciary_collateral_value %s, got %s", judiciary, p.JudiciaryCollateralValue())
	}
	if !p.CreatedAt().Equal(created) {
		t.Fatalf("expected created_at %v, got %v", created, p.CreatedAt())
	}
	if !p.UpdatedAt().Equal(updated) {
		t.Fatalf("expected updated_at %v, got %v", updated, p.UpdatedAt())
	}
	if !p.PurchasedAt().Equal(purchased) {
		t.Fatalf("expected purchased_at %v, got %v", purchased, p.PurchasedAt())
	}
	if p.RowVersion() != 3 {
		t.Fatalf("expected row_version 3, got %d", p.RowVersion())
	}
}

func TestReconstructPosition_InconsistentTotalValue(t *testing.T) {
	amount := decimal.NewFromInt(10)
	unitPrice := decimal.NewFromInt(5)
	wrongTotal := decimal.NewFromInt(999)

	_, err := domain.ReconstructPosition(
		uuid.New(), uuid.New(), uuid.New(),
		amount, unitPrice, wrongTotal,
		decimal.Zero, decimal.Zero,
		time.Now(), time.Now(), time.Now(), 1,
	)
	if err == nil {
		t.Fatal("expected error for inconsistent total_value")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestReconstructPosition_NegativeAmount(t *testing.T) {
	_, err := domain.ReconstructPosition(
		uuid.New(), uuid.New(), uuid.New(),
		decimal.NewFromInt(-1), decimal.NewFromInt(5), decimal.NewFromInt(-5),
		decimal.Zero, decimal.Zero,
		time.Now(), time.Now(), time.Now(), 1,
	)
	if err == nil {
		t.Fatal("expected error for negative amount")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestReconstructPosition_NegativeUnitPrice(t *testing.T) {
	_, err := domain.ReconstructPosition(
		uuid.New(), uuid.New(), uuid.New(),
		decimal.NewFromInt(10), decimal.NewFromInt(-5), decimal.NewFromInt(-50),
		decimal.Zero, decimal.Zero,
		time.Now(), time.Now(), time.Now(), 1,
	)
	if err == nil {
		t.Fatal("expected error for negative unit_price")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestReconstructPosition_NegativeCollateral(t *testing.T) {
	amount := decimal.NewFromInt(10)
	unitPrice := decimal.NewFromInt(5)
	totalValue := amount.Mul(unitPrice)

	_, err := domain.ReconstructPosition(
		uuid.New(), uuid.New(), uuid.New(),
		amount, unitPrice, totalValue,
		decimal.NewFromInt(-1), decimal.Zero,
		time.Now(), time.Now(), time.Now(), 1,
	)
	if err == nil {
		t.Fatal("expected error for negative collateral_value")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestReconstructPosition_NegativeJudiciaryCollateral(t *testing.T) {
	amount := decimal.NewFromInt(10)
	unitPrice := decimal.NewFromInt(5)
	totalValue := amount.Mul(unitPrice)

	_, err := domain.ReconstructPosition(
		uuid.New(), uuid.New(), uuid.New(),
		amount, unitPrice, totalValue,
		decimal.Zero, decimal.NewFromInt(-1),
		time.Now(), time.Now(), time.Now(), 1,
	)
	if err == nil {
		t.Fatal("expected error for negative judiciary_collateral_value")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestPosition_DecimalPrecision(t *testing.T) {
	amount := decimal.RequireFromString("123.456789")
	unitPrice := decimal.RequireFromString("98.76543210")
	expected := amount.Mul(unitPrice)

	p, err := domain.NewPosition(uuid.New(), uuid.New(), amount, unitPrice, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.TotalValue().Equal(expected) {
		t.Fatalf("expected total_value %s, got %s", expected, p.TotalValue())
	}
}

func TestPosition_DecimalPrecision_NoFloatingPointDrift(t *testing.T) {
	amount := decimal.RequireFromString("0.1")
	unitPrice := decimal.RequireFromString("0.2")
	expected := decimal.RequireFromString("0.02")

	p, err := domain.NewPosition(uuid.New(), uuid.New(), amount, unitPrice, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.TotalValue().Equal(expected) {
		t.Fatalf("expected total_value %s, got %s (floating-point drift detected)", expected, p.TotalValue())
	}
}
