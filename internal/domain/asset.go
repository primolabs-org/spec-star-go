package domain

import (
	"time"

	"github.com/google/uuid"
)

// Asset represents a tradable fixed-income instrument.
type Asset struct {
	assetID          uuid.UUID
	instrumentID     string
	productType      ProductType
	offerID          string
	emissionEntityID string
	issuerDocumentID string
	marketCode       string
	assetName        string
	issuanceDate     time.Time
	maturityDate     time.Time
	createdAt        time.Time
}

// NewAsset creates an Asset with a generated UUID and current timestamp.
func NewAsset(
	instrumentID string,
	productType ProductType,
	offerID string,
	emissionEntityID string,
	issuerDocumentID string,
	marketCode string,
	assetName string,
	issuanceDate time.Time,
	maturityDate time.Time,
) (*Asset, error) {
	if err := validateAssetFields(instrumentID, productType, emissionEntityID, assetName, issuanceDate, maturityDate); err != nil {
		return nil, err
	}
	return &Asset{
		assetID:          uuid.New(),
		instrumentID:     instrumentID,
		productType:      productType,
		offerID:          offerID,
		emissionEntityID: emissionEntityID,
		issuerDocumentID: issuerDocumentID,
		marketCode:       marketCode,
		assetName:        assetName,
		issuanceDate:     issuanceDate,
		maturityDate:     maturityDate,
		createdAt:        time.Now(),
	}, nil
}

func validateAssetFields(
	instrumentID string,
	productType ProductType,
	emissionEntityID string,
	assetName string,
	issuanceDate time.Time,
	maturityDate time.Time,
) error {
	if instrumentID == "" {
		return &ValidationError{Message: "instrument_id is required"}
	}
	if err := ValidateProductType(productType); err != nil {
		return err
	}
	if emissionEntityID == "" {
		return &ValidationError{Message: "emission_entity_id is required"}
	}
	if assetName == "" {
		return &ValidationError{Message: "asset_name is required"}
	}
	if issuanceDate.IsZero() {
		return &ValidationError{Message: "issuance_date is required"}
	}
	if maturityDate.IsZero() {
		return &ValidationError{Message: "maturity_date is required"}
	}
	return nil
}

// ReconstructAsset loads an Asset from persisted fields without generating new values.
func ReconstructAsset(
	assetID uuid.UUID,
	instrumentID string,
	productType ProductType,
	offerID string,
	emissionEntityID string,
	issuerDocumentID string,
	marketCode string,
	assetName string,
	issuanceDate time.Time,
	maturityDate time.Time,
	createdAt time.Time,
) *Asset {
	return &Asset{
		assetID:          assetID,
		instrumentID:     instrumentID,
		productType:      productType,
		offerID:          offerID,
		emissionEntityID: emissionEntityID,
		issuerDocumentID: issuerDocumentID,
		marketCode:       marketCode,
		assetName:        assetName,
		issuanceDate:     issuanceDate,
		maturityDate:     maturityDate,
		createdAt:        createdAt,
	}
}

func (a *Asset) AssetID() uuid.UUID      { return a.assetID }
func (a *Asset) InstrumentID() string     { return a.instrumentID }
func (a *Asset) ProductType() ProductType { return a.productType }
func (a *Asset) OfferID() string          { return a.offerID }
func (a *Asset) EmissionEntityID() string { return a.emissionEntityID }
func (a *Asset) IssuerDocumentID() string { return a.issuerDocumentID }
func (a *Asset) MarketCode() string       { return a.marketCode }
func (a *Asset) AssetName() string        { return a.assetName }
func (a *Asset) IssuanceDate() time.Time  { return a.issuanceDate }
func (a *Asset) MaturityDate() time.Time  { return a.maturityDate }
func (a *Asset) CreatedAt() time.Time     { return a.createdAt }
