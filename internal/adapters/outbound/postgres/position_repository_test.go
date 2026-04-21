//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/shopspring/decimal"
)

func TestPositionRepository_CreateAndFindByID(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	exporter := setupPostgresTestTracer(t)
	clientRepo := NewClientRepository(pool)
	assetRepo := NewAssetRepository(pool)
	posRepo := NewPositionRepository(pool)

	client := createTestClient(t, ctx, clientRepo)
	asset := createTestAsset(t, ctx, assetRepo, "INSTR-POS-"+uuid.NewString()[:8])

	purchasedAt := time.Now().Truncate(time.Microsecond)
	pos, err := domain.NewPosition(
		client.ClientID(), asset.AssetID(),
		decimal.RequireFromString("100.500000"),
		decimal.RequireFromString("10.12345678"),
		purchasedAt,
	)
	if err != nil {
		t.Fatalf("creating position: %v", err)
	}
	if err := posRepo.Create(ctx, pos); err != nil {
		t.Fatalf("inserting position: %v", err)
	}

	got, err := posRepo.FindByID(ctx, pos.PositionID())
	if err != nil {
		t.Fatalf("finding position: %v", err)
	}

	if got.PositionID() != pos.PositionID() {
		t.Errorf("position_id: got %s, want %s", got.PositionID(), pos.PositionID())
	}
	if got.ClientID() != pos.ClientID() {
		t.Errorf("client_id: got %s, want %s", got.ClientID(), pos.ClientID())
	}
	if got.AssetID() != pos.AssetID() {
		t.Errorf("asset_id: got %s, want %s", got.AssetID(), pos.AssetID())
	}
	if !got.Amount().Equal(pos.Amount()) {
		t.Errorf("amount: got %s, want %s", got.Amount(), pos.Amount())
	}
	if !got.UnitPrice().Equal(pos.UnitPrice()) {
		t.Errorf("unit_price: got %s, want %s", got.UnitPrice(), pos.UnitPrice())
	}
	if !got.TotalValue().Equal(pos.TotalValue()) {
		t.Errorf("total_value: got %s, want %s", got.TotalValue(), pos.TotalValue())
	}
	if !got.CollateralValue().Equal(pos.CollateralValue()) {
		t.Errorf("collateral_value: got %s, want %s", got.CollateralValue(), pos.CollateralValue())
	}
	if !got.JudiciaryCollateralValue().Equal(pos.JudiciaryCollateralValue()) {
		t.Errorf("judiciary_collateral_value: got %s, want %s", got.JudiciaryCollateralValue(), pos.JudiciaryCollateralValue())
	}
	if got.RowVersion() != pos.RowVersion() {
		t.Errorf("row_version: got %d, want %d", got.RowVersion(), pos.RowVersion())
	}
	if !timesEqualMicro(got.PurchasedAt(), purchasedAt) {
		t.Errorf("purchased_at: got %v, want %v", got.PurchasedAt(), purchasedAt)
	}

	requireDBSpanSuccess(t, exporter, "db.position.create", "INSERT")
}

func TestPositionRepository_FindByClientAndAsset(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	clientRepo := NewClientRepository(pool)
	assetRepo := NewAssetRepository(pool)
	posRepo := NewPositionRepository(pool)

	client := createTestClient(t, ctx, clientRepo)
	asset := createTestAsset(t, ctx, assetRepo, "INSTR-CA-"+uuid.NewString()[:8])

	pos1, err := domain.NewPosition(client.ClientID(), asset.AssetID(),
		decimal.RequireFromString("50"), decimal.RequireFromString("10"), time.Now())
	if err != nil {
		t.Fatalf("creating position 1: %v", err)
	}
	pos2, err := domain.NewPosition(client.ClientID(), asset.AssetID(),
		decimal.RequireFromString("75"), decimal.RequireFromString("20"), time.Now())
	if err != nil {
		t.Fatalf("creating position 2: %v", err)
	}
	if err := posRepo.Create(ctx, pos1); err != nil {
		t.Fatalf("inserting position 1: %v", err)
	}
	if err := posRepo.Create(ctx, pos2); err != nil {
		t.Fatalf("inserting position 2: %v", err)
	}

	got, err := posRepo.FindByClientAndAsset(ctx, client.ClientID(), asset.AssetID())
	if err != nil {
		t.Fatalf("finding positions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(got))
	}
}

func TestPositionRepository_FindByClientAndInstrument(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	exporter := setupPostgresTestTracer(t)
	clientRepo := NewClientRepository(pool)
	assetRepo := NewAssetRepository(pool)
	posRepo := NewPositionRepository(pool)

	client := createTestClient(t, ctx, clientRepo)
	instrumentID := "INSTR-CI-" + uuid.NewString()[:8]
	asset := createTestAsset(t, ctx, assetRepo, instrumentID)

	earlier := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	later := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)

	pos1, err := domain.NewPosition(client.ClientID(), asset.AssetID(),
		decimal.RequireFromString("30"), decimal.RequireFromString("15"), later)
	if err != nil {
		t.Fatalf("creating position 1: %v", err)
	}
	pos2, err := domain.NewPosition(client.ClientID(), asset.AssetID(),
		decimal.RequireFromString("40"), decimal.RequireFromString("25"), earlier)
	if err != nil {
		t.Fatalf("creating position 2: %v", err)
	}
	if err := posRepo.Create(ctx, pos1); err != nil {
		t.Fatalf("inserting position 1: %v", err)
	}
	if err := posRepo.Create(ctx, pos2); err != nil {
		t.Fatalf("inserting position 2: %v", err)
	}

	got, err := posRepo.FindByClientAndInstrument(ctx, client.ClientID(), instrumentID)
	if err != nil {
		t.Fatalf("finding positions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(got))
	}
	if got[0].PurchasedAt().After(got[1].PurchasedAt()) {
		t.Error("positions not ordered by purchased_at ascending")
	}

	requireDBSpanSuccess(t, exporter, "db.position.find_by_client_and_instrument", "SELECT")
}

func TestPositionRepository_FindByClientAndInstrument_ContextCanceled(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	exporter := setupPostgresTestTracer(t)
	posRepo := NewPositionRepository(pool)

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err := posRepo.FindByClientAndInstrument(canceledCtx, uuid.New(), "INSTR-CANCELED")
	if err == nil {
		t.Fatal("expected error with canceled context")
	}

	requireDBSpanError(t, exporter, "db.position.find_by_client_and_instrument", "SELECT")
}

func TestPositionRepository_FindByClientAndInstrument_NoMatches(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	posRepo := NewPositionRepository(pool)

	got, err := posRepo.FindByClientAndInstrument(ctx, uuid.New(), "NONEXISTENT-"+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d positions", len(got))
	}
}

func TestPositionRepository_Create_DuplicatePositionID(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	exporter := setupPostgresTestTracer(t)
	clientRepo := NewClientRepository(pool)
	assetRepo := NewAssetRepository(pool)
	posRepo := NewPositionRepository(pool)

	client := createTestClient(t, ctx, clientRepo)
	asset := createTestAsset(t, ctx, assetRepo, "INSTR-DUP-POS-"+uuid.NewString()[:8])

	pos, err := domain.NewPosition(
		client.ClientID(),
		asset.AssetID(),
		decimal.RequireFromString("100"),
		decimal.RequireFromString("10"),
		time.Now(),
	)
	if err != nil {
		t.Fatalf("creating position: %v", err)
	}

	if err := posRepo.Create(ctx, pos); err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}

	err = posRepo.Create(ctx, pos)
	if err == nil {
		t.Fatal("expected duplicate insert error")
	}

	requireDBSpanError(t, exporter, "db.position.create", "INSERT")
}

func TestPositionRepository_Update_Success(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	exporter := setupPostgresTestTracer(t)
	clientRepo := NewClientRepository(pool)
	assetRepo := NewAssetRepository(pool)
	posRepo := NewPositionRepository(pool)

	client := createTestClient(t, ctx, clientRepo)
	asset := createTestAsset(t, ctx, assetRepo, "INSTR-UP-"+uuid.NewString()[:8])

	pos, err := domain.NewPosition(client.ClientID(), asset.AssetID(),
		decimal.RequireFromString("100"), decimal.RequireFromString("10.50"), time.Now())
	if err != nil {
		t.Fatalf("creating position: %v", err)
	}
	if err := posRepo.Create(ctx, pos); err != nil {
		t.Fatalf("inserting position: %v", err)
	}

	loaded, err := posRepo.FindByID(ctx, pos.PositionID())
	if err != nil {
		t.Fatalf("finding position: %v", err)
	}
	if err := loaded.UpdateAmount(decimal.RequireFromString("50.250000")); err != nil {
		t.Fatalf("updating amount: %v", err)
	}
	if err := posRepo.Update(ctx, loaded); err != nil {
		t.Fatalf("updating position: %v", err)
	}

	reloaded, err := posRepo.FindByID(ctx, pos.PositionID())
	if err != nil {
		t.Fatalf("reloading position: %v", err)
	}
	if !reloaded.Amount().Equal(decimal.RequireFromString("50.250000")) {
		t.Errorf("amount after update: got %s, want 50.250000", reloaded.Amount())
	}
	if reloaded.RowVersion() != 2 {
		t.Errorf("row_version after update: got %d, want 2", reloaded.RowVersion())
	}

	requireDBSpanSuccess(t, exporter, "db.position.update", "UPDATE")
}

func TestPositionRepository_Update_ConcurrencyConflict(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	exporter := setupPostgresTestTracer(t)
	clientRepo := NewClientRepository(pool)
	assetRepo := NewAssetRepository(pool)
	posRepo := NewPositionRepository(pool)

	client := createTestClient(t, ctx, clientRepo)
	asset := createTestAsset(t, ctx, assetRepo, "INSTR-CC-"+uuid.NewString()[:8])

	pos, err := domain.NewPosition(client.ClientID(), asset.AssetID(),
		decimal.RequireFromString("100"), decimal.RequireFromString("10"), time.Now())
	if err != nil {
		t.Fatalf("creating position: %v", err)
	}
	if err := posRepo.Create(ctx, pos); err != nil {
		t.Fatalf("inserting position: %v", err)
	}

	posA, err := posRepo.FindByID(ctx, pos.PositionID())
	if err != nil {
		t.Fatalf("reading position A: %v", err)
	}
	posB, err := posRepo.FindByID(ctx, pos.PositionID())
	if err != nil {
		t.Fatalf("reading position B: %v", err)
	}

	if err := posA.UpdateAmount(decimal.RequireFromString("50")); err != nil {
		t.Fatalf("mutating position A: %v", err)
	}
	if err := posB.UpdateAmount(decimal.RequireFromString("75")); err != nil {
		t.Fatalf("mutating position B: %v", err)
	}

	if err := posRepo.Update(ctx, posA); err != nil {
		t.Fatalf("first update should succeed: %v", err)
	}

	err = posRepo.Update(ctx, posB)
	if err == nil {
		t.Fatal("expected concurrency conflict error")
	}
	if !errors.Is(err, domain.ErrConcurrencyConflict) {
		t.Errorf("expected ErrConcurrencyConflict, got: %v", err)
	}

	requireDBSpanError(t, exporter, "db.position.update", "UPDATE")
}

func TestPositionRepository_FindByID_NotFound(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	posRepo := NewPositionRepository(pool)

	_, err := posRepo.FindByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent position")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestPositionRepository_DecimalPrecision(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	clientRepo := NewClientRepository(pool)
	assetRepo := NewAssetRepository(pool)
	posRepo := NewPositionRepository(pool)

	client := createTestClient(t, ctx, clientRepo)
	asset := createTestAsset(t, ctx, assetRepo, "INSTR-DEC-"+uuid.NewString()[:8])

	amount := decimal.RequireFromString("100.000000")
	unitPrice := decimal.RequireFromString("99.12345678")
	expectedTotal := amount.Mul(unitPrice)

	pos, err := domain.NewPosition(client.ClientID(), asset.AssetID(), amount, unitPrice, time.Now())
	if err != nil {
		t.Fatalf("creating position: %v", err)
	}
	if err := posRepo.Create(ctx, pos); err != nil {
		t.Fatalf("inserting position: %v", err)
	}

	got, err := posRepo.FindByID(ctx, pos.PositionID())
	if err != nil {
		t.Fatalf("finding position: %v", err)
	}

	if !got.Amount().Equal(amount) {
		t.Errorf("amount: got %s, want %s", got.Amount(), amount)
	}
	if !got.UnitPrice().Equal(unitPrice) {
		t.Errorf("unit_price: got %s, want %s", got.UnitPrice(), unitPrice)
	}
	if !got.TotalValue().Equal(expectedTotal) {
		t.Errorf("total_value: got %s, want %s (amount × unit_price)", got.TotalValue(), expectedTotal)
	}
}
