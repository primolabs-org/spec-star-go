package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/primolabs-org/spec-star-go/internal/platform"
	"github.com/primolabs-org/spec-star-go/internal/ports"
)

var _ ports.AssetRepository = (*AssetRepository)(nil)

type AssetRepository struct {
	pool *pgxpool.Pool
}

func NewAssetRepository(pool *pgxpool.Pool) *AssetRepository {
	return &AssetRepository{pool: pool}
}

const assetColumns = `asset_id, instrument_id, product_type, offer_id, emission_entity_id,
	issuer_document_id, market_code, asset_name, issuance_date, maturity_date, created_at`

func (r *AssetRepository) FindByID(ctx context.Context, assetID uuid.UUID) (*domain.Asset, error) {
	db := executorFromContext(ctx, r.pool)
	asset, err := scanAsset(
		db.QueryRow(ctx, `SELECT `+assetColumns+` FROM assets WHERE asset_id = $1`, assetID),
		fmt.Sprintf("asset %s", assetID),
	)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			platform.LoggerFromContext(ctx).Error("FindByID: query failed", "asset_id", assetID.String(), "error", err.Error())
		}
		return nil, err
	}
	return asset, nil
}

func (r *AssetRepository) FindByInstrumentID(ctx context.Context, instrumentID string) (*domain.Asset, error) {
	db := executorFromContext(ctx, r.pool)
	asset, err := scanAsset(
		db.QueryRow(ctx, `SELECT `+assetColumns+` FROM assets WHERE instrument_id = $1`, instrumentID),
		fmt.Sprintf("asset with instrument_id %s", instrumentID),
	)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			platform.LoggerFromContext(ctx).Error("FindByInstrumentID: query failed", "instrument_id", instrumentID, "error", err.Error())
		}
		return nil, err
	}
	return asset, nil
}

func scanAsset(row pgx.Row, label string) (*domain.Asset, error) {
	var (
		assetID          uuid.UUID
		instrumentID     string
		productType      string
		offerID          *string
		emissionEntityID string
		issuerDocumentID *string
		marketCode       *string
		assetName        string
		issuanceDate     time.Time
		maturityDate     time.Time
		createdAt        time.Time
	)
	err := row.Scan(
		&assetID, &instrumentID, &productType, &offerID, &emissionEntityID,
		&issuerDocumentID, &marketCode, &assetName, &issuanceDate, &maturityDate, &createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", label, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("querying %s: %w", label, err)
	}

	return domain.ReconstructAsset(
		assetID,
		instrumentID,
		domain.ProductType(productType),
		stringFromNullable(offerID),
		emissionEntityID,
		stringFromNullable(issuerDocumentID),
		stringFromNullable(marketCode),
		assetName,
		issuanceDate,
		maturityDate,
		createdAt,
	), nil
}

func (r *AssetRepository) Create(ctx context.Context, asset *domain.Asset) error {
	db := executorFromContext(ctx, r.pool)

	_, err := db.Exec(ctx,
		`INSERT INTO assets (asset_id, instrument_id, product_type, offer_id, emission_entity_id,
			issuer_document_id, market_code, asset_name, issuance_date, maturity_date, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		asset.AssetID(), asset.InstrumentID(), string(asset.ProductType()),
		nullableString(asset.OfferID()), asset.EmissionEntityID(),
		nullableString(asset.IssuerDocumentID()), nullableString(asset.MarketCode()),
		asset.AssetName(), asset.IssuanceDate(), asset.MaturityDate(), asset.CreatedAt(),
	)
	if err != nil {
		platform.LoggerFromContext(ctx).Error("Create: exec failed", "asset_id", asset.AssetID().String(), "error", err.Error())
		return fmt.Errorf("inserting asset %s: %w", asset.AssetID(), err)
	}
	return nil
}
