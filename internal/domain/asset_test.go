package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
)

func validAssetDates() (time.Time, time.Time) {
	return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func TestNewAsset_ValidCreation(t *testing.T) {
	issuance, maturity := validAssetDates()
	before := time.Now()
	a, err := domain.NewAsset("INST-001", domain.ProductTypeCDB, "OFFER-1", "EMIT-1", "DOC-1", "MKT-1", "CDB Prefixado", issuance, maturity)
	after := time.Now()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.AssetID() == uuid.Nil {
		t.Fatal("expected non-nil asset_id")
	}
	if a.InstrumentID() != "INST-001" {
		t.Fatalf("expected instrument_id %q, got %q", "INST-001", a.InstrumentID())
	}
	if a.ProductType() != domain.ProductTypeCDB {
		t.Fatalf("expected product_type %q, got %q", domain.ProductTypeCDB, a.ProductType())
	}
	if a.OfferID() != "OFFER-1" {
		t.Fatalf("expected offer_id %q, got %q", "OFFER-1", a.OfferID())
	}
	if a.EmissionEntityID() != "EMIT-1" {
		t.Fatalf("expected emission_entity_id %q, got %q", "EMIT-1", a.EmissionEntityID())
	}
	if a.IssuerDocumentID() != "DOC-1" {
		t.Fatalf("expected issuer_document_id %q, got %q", "DOC-1", a.IssuerDocumentID())
	}
	if a.MarketCode() != "MKT-1" {
		t.Fatalf("expected market_code %q, got %q", "MKT-1", a.MarketCode())
	}
	if a.AssetName() != "CDB Prefixado" {
		t.Fatalf("expected asset_name %q, got %q", "CDB Prefixado", a.AssetName())
	}
	if !a.IssuanceDate().Equal(issuance) {
		t.Fatalf("expected issuance_date %v, got %v", issuance, a.IssuanceDate())
	}
	if !a.MaturityDate().Equal(maturity) {
		t.Fatalf("expected maturity_date %v, got %v", maturity, a.MaturityDate())
	}
	if a.CreatedAt().Before(before) || a.CreatedAt().After(after) {
		t.Fatalf("expected created_at between %v and %v, got %v", before, after, a.CreatedAt())
	}
}

func TestNewAsset_InvalidProductType(t *testing.T) {
	issuance, maturity := validAssetDates()
	_, err := domain.NewAsset("INST-001", "INVALID", "", "EMIT-1", "", "", "Name", issuance, maturity)
	if err == nil {
		t.Fatal("expected error for invalid product_type")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewAsset_MissingInstrumentID(t *testing.T) {
	issuance, maturity := validAssetDates()
	_, err := domain.NewAsset("", domain.ProductTypeCDB, "", "EMIT-1", "", "", "Name", issuance, maturity)
	if err == nil {
		t.Fatal("expected error for empty instrument_id")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewAsset_MissingEmissionEntityID(t *testing.T) {
	issuance, maturity := validAssetDates()
	_, err := domain.NewAsset("INST-001", domain.ProductTypeCDB, "", "", "", "", "Name", issuance, maturity)
	if err == nil {
		t.Fatal("expected error for empty emission_entity_id")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewAsset_MissingAssetName(t *testing.T) {
	issuance, maturity := validAssetDates()
	_, err := domain.NewAsset("INST-001", domain.ProductTypeCDB, "", "EMIT-1", "", "", "", issuance, maturity)
	if err == nil {
		t.Fatal("expected error for empty asset_name")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewAsset_MissingIssuanceDate(t *testing.T) {
	_, maturity := validAssetDates()
	_, err := domain.NewAsset("INST-001", domain.ProductTypeCDB, "", "EMIT-1", "", "", "Name", time.Time{}, maturity)
	if err == nil {
		t.Fatal("expected error for zero issuance_date")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewAsset_MissingMaturityDate(t *testing.T) {
	issuance, _ := validAssetDates()
	_, err := domain.NewAsset("INST-001", domain.ProductTypeCDB, "", "EMIT-1", "", "", "Name", issuance, time.Time{})
	if err == nil {
		t.Fatal("expected error for zero maturity_date")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewAsset_OptionalFieldsAcceptedAsEmpty(t *testing.T) {
	issuance, maturity := validAssetDates()
	a, err := domain.NewAsset("INST-001", domain.ProductTypeCDB, "", "EMIT-1", "", "", "Name", issuance, maturity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.OfferID() != "" {
		t.Fatalf("expected empty offer_id, got %q", a.OfferID())
	}
	if a.IssuerDocumentID() != "" {
		t.Fatalf("expected empty issuer_document_id, got %q", a.IssuerDocumentID())
	}
	if a.MarketCode() != "" {
		t.Fatalf("expected empty market_code, got %q", a.MarketCode())
	}
}

func TestReconstructAsset(t *testing.T) {
	id := uuid.New()
	issuance, maturity := validAssetDates()
	created := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	a := domain.ReconstructAsset(id, "INST-001", domain.ProductTypeLF, "OFFER-1", "EMIT-1", "DOC-1", "MKT-1", "LF Post", issuance, maturity, created)

	if a.AssetID() != id {
		t.Fatalf("expected asset_id %v, got %v", id, a.AssetID())
	}
	if a.InstrumentID() != "INST-001" {
		t.Fatalf("expected instrument_id %q, got %q", "INST-001", a.InstrumentID())
	}
	if a.ProductType() != domain.ProductTypeLF {
		t.Fatalf("expected product_type %q, got %q", domain.ProductTypeLF, a.ProductType())
	}
	if a.OfferID() != "OFFER-1" {
		t.Fatalf("expected offer_id %q, got %q", "OFFER-1", a.OfferID())
	}
	if a.EmissionEntityID() != "EMIT-1" {
		t.Fatalf("expected emission_entity_id %q, got %q", "EMIT-1", a.EmissionEntityID())
	}
	if a.IssuerDocumentID() != "DOC-1" {
		t.Fatalf("expected issuer_document_id %q, got %q", "DOC-1", a.IssuerDocumentID())
	}
	if a.MarketCode() != "MKT-1" {
		t.Fatalf("expected market_code %q, got %q", "MKT-1", a.MarketCode())
	}
	if a.AssetName() != "LF Post" {
		t.Fatalf("expected asset_name %q, got %q", "LF Post", a.AssetName())
	}
	if !a.IssuanceDate().Equal(issuance) {
		t.Fatalf("expected issuance_date %v, got %v", issuance, a.IssuanceDate())
	}
	if !a.MaturityDate().Equal(maturity) {
		t.Fatalf("expected maturity_date %v, got %v", maturity, a.MaturityDate())
	}
	if !a.CreatedAt().Equal(created) {
		t.Fatalf("expected created_at %v, got %v", created, a.CreatedAt())
	}
}
