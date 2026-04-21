//go:build integration

package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
)

func TestAssetRepository_CreateAndFindByID(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	exporter := setupPostgresTestTracer(t)
	repo := NewAssetRepository(pool)

	asset, err := domain.NewAsset(
		"INSTR-001",
		domain.ProductTypeCDB,
		"OFFER-001",
		"EMITTER-001",
		"ISSUER-DOC-001",
		"MKTCODE-001",
		"Test CDB Asset",
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("creating asset: %v", err)
	}
	if err := repo.Create(ctx, asset); err != nil {
		t.Fatalf("inserting asset: %v", err)
	}

	got, err := repo.FindByID(ctx, asset.AssetID())
	if err != nil {
		t.Fatalf("finding asset: %v", err)
	}

	if got.AssetID() != asset.AssetID() {
		t.Errorf("asset_id: got %s, want %s", got.AssetID(), asset.AssetID())
	}
	if got.InstrumentID() != asset.InstrumentID() {
		t.Errorf("instrument_id: got %q, want %q", got.InstrumentID(), asset.InstrumentID())
	}
	if got.ProductType() != asset.ProductType() {
		t.Errorf("product_type: got %q, want %q", got.ProductType(), asset.ProductType())
	}
	if got.OfferID() != asset.OfferID() {
		t.Errorf("offer_id: got %q, want %q", got.OfferID(), asset.OfferID())
	}
	if got.EmissionEntityID() != asset.EmissionEntityID() {
		t.Errorf("emission_entity_id: got %q, want %q", got.EmissionEntityID(), asset.EmissionEntityID())
	}
	if got.IssuerDocumentID() != asset.IssuerDocumentID() {
		t.Errorf("issuer_document_id: got %q, want %q", got.IssuerDocumentID(), asset.IssuerDocumentID())
	}
	if got.MarketCode() != asset.MarketCode() {
		t.Errorf("market_code: got %q, want %q", got.MarketCode(), asset.MarketCode())
	}
	if got.AssetName() != asset.AssetName() {
		t.Errorf("asset_name: got %q, want %q", got.AssetName(), asset.AssetName())
	}
	if !datesEqual(got.IssuanceDate(), asset.IssuanceDate()) {
		t.Errorf("issuance_date: got %v, want %v", got.IssuanceDate(), asset.IssuanceDate())
	}
	if !datesEqual(got.MaturityDate(), asset.MaturityDate()) {
		t.Errorf("maturity_date: got %v, want %v", got.MaturityDate(), asset.MaturityDate())
	}
	if !timesEqualMicro(got.CreatedAt(), asset.CreatedAt()) {
		t.Errorf("created_at: got %v, want %v", got.CreatedAt(), asset.CreatedAt())
	}

	requireDBSpanSuccess(t, exporter, "db.asset.find_by_id", "SELECT")
}

func TestAssetRepository_FindByInstrumentID(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	repo := NewAssetRepository(pool)

	instrumentID := "INSTR-FIND-" + uuid.NewString()[:8]
	asset := createTestAsset(t, ctx, repo, instrumentID)

	got, err := repo.FindByInstrumentID(ctx, instrumentID)
	if err != nil {
		t.Fatalf("finding asset by instrument_id: %v", err)
	}
	if got.AssetID() != asset.AssetID() {
		t.Errorf("asset_id: got %s, want %s", got.AssetID(), asset.AssetID())
	}
}

func TestAssetRepository_FindByID_NotFound(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	exporter := setupPostgresTestTracer(t)
	repo := NewAssetRepository(pool)

	_, err := repo.FindByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent asset")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}

	requireDBSpanError(t, exporter, "db.asset.find_by_id", "SELECT")
}

func TestAssetRepository_FindByInstrumentID_NotFound(t *testing.T) {
	pool := testPool(t)
	ctx := withTestTx(t, pool)
	repo := NewAssetRepository(pool)

	_, err := repo.FindByInstrumentID(ctx, "NONEXISTENT-"+uuid.NewString()[:8])
	if err == nil {
		t.Fatal("expected error for non-existent instrument_id")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}
