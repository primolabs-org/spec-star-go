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
	"github.com/shopspring/decimal"
)

var _ ports.PositionRepository = (*PositionRepository)(nil)

type PositionRepository struct {
	pool *pgxpool.Pool
}

func NewPositionRepository(pool *pgxpool.Pool) *PositionRepository {
	return &PositionRepository{pool: pool}
}

const positionColumns = `position_id, client_id, asset_id, amount, unit_price, total_value,
	collateral_value, judiciary_collateral_value, created_at, updated_at, purchased_at, row_version`

func (r *PositionRepository) FindByID(ctx context.Context, positionID uuid.UUID) (*domain.Position, error) {
	db := executorFromContext(ctx, r.pool)
	pos, err := scanPosition(db.QueryRow(ctx,
		`SELECT `+positionColumns+` FROM positions WHERE position_id = $1`,
		positionID,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("position %s: %w", positionID, domain.ErrNotFound)
		}
		platform.LoggerFromContext(ctx).Error("FindByID: query failed", "position_id", positionID.String(), "error", err.Error())
		return nil, fmt.Errorf("querying position %s: %w", positionID, err)
	}
	return pos, nil
}

func (r *PositionRepository) FindByClientAndAsset(ctx context.Context, clientID, assetID uuid.UUID) ([]*domain.Position, error) {
	db := executorFromContext(ctx, r.pool)
	rows, err := db.Query(ctx,
		`SELECT `+positionColumns+` FROM positions WHERE client_id = $1 AND asset_id = $2`,
		clientID, assetID,
	)
	if err != nil {
		platform.LoggerFromContext(ctx).Error("FindByClientAndAsset: query failed", "client_id", clientID.String(), "asset_id", assetID.String(), "error", err.Error())
		return nil, fmt.Errorf("querying positions for client %s asset %s: %w", clientID, assetID, err)
	}
	defer rows.Close()
	return collectPositions(rows)
}

func (r *PositionRepository) FindByClientAndInstrument(ctx context.Context, clientID uuid.UUID, instrumentID string) ([]*domain.Position, error) {
	db := executorFromContext(ctx, r.pool)
	rows, err := db.Query(ctx,
		`SELECT p.position_id, p.client_id, p.asset_id, p.amount, p.unit_price, p.total_value,
			p.collateral_value, p.judiciary_collateral_value, p.created_at, p.updated_at,
			p.purchased_at, p.row_version
		 FROM positions p
		 JOIN assets a ON p.asset_id = a.asset_id
		 WHERE p.client_id = $1 AND a.instrument_id = $2
		 ORDER BY p.purchased_at ASC`,
		clientID, instrumentID,
	)
	if err != nil {
		platform.LoggerFromContext(ctx).Error("FindByClientAndInstrument: query failed", "client_id", clientID.String(), "instrument_id", instrumentID, "error", err.Error())
		return nil, fmt.Errorf("querying positions for client %s instrument %s: %w", clientID, instrumentID, err)
	}
	defer rows.Close()
	return collectPositions(rows)
}

func (r *PositionRepository) Create(ctx context.Context, position *domain.Position) error {
	db := executorFromContext(ctx, r.pool)
	_, err := db.Exec(ctx,
		`INSERT INTO positions (`+positionColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		position.PositionID(), position.ClientID(), position.AssetID(),
		position.Amount().String(), position.UnitPrice().String(), position.TotalValue().String(),
		position.CollateralValue().String(), position.JudiciaryCollateralValue().String(),
		position.CreatedAt(), position.UpdatedAt(), position.PurchasedAt(), position.RowVersion(),
	)
	if err != nil {
		platform.LoggerFromContext(ctx).Error("Create: exec failed", "position_id", position.PositionID().String(), "error", err.Error())
		return fmt.Errorf("inserting position %s: %w", position.PositionID(), err)
	}
	return nil
}

func (r *PositionRepository) Update(ctx context.Context, position *domain.Position) error {
	db := executorFromContext(ctx, r.pool)
	tag, err := db.Exec(ctx,
		`UPDATE positions SET amount = $1, unit_price = $2, total_value = $3, collateral_value = $4,
		 judiciary_collateral_value = $5, updated_at = $6, row_version = $7
		 WHERE position_id = $8 AND row_version = $9`,
		position.Amount().String(), position.UnitPrice().String(), position.TotalValue().String(),
		position.CollateralValue().String(), position.JudiciaryCollateralValue().String(),
		position.UpdatedAt(), position.RowVersion(),
		position.PositionID(), position.RowVersion()-1,
	)
	if err != nil {
		platform.LoggerFromContext(ctx).Error("Update: exec failed", "position_id", position.PositionID().String(), "error", err.Error())
		return fmt.Errorf("updating position %s: %w", position.PositionID(), err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("position %s: %w", position.PositionID(), domain.ErrConcurrencyConflict)
	}
	return nil
}

func scanPosition(row pgx.Row) (*domain.Position, error) {
	var (
		positionID               uuid.UUID
		clientID                 uuid.UUID
		assetID                  uuid.UUID
		amount                   string
		unitPrice                string
		totalValue               string
		collateralValue          string
		judiciaryCollateralValue string
		createdAt                time.Time
		updatedAt                time.Time
		purchasedAt              time.Time
		rowVersion               int
	)
	if err := row.Scan(
		&positionID, &clientID, &assetID, &amount, &unitPrice, &totalValue,
		&collateralValue, &judiciaryCollateralValue, &createdAt, &updatedAt,
		&purchasedAt, &rowVersion,
	); err != nil {
		return nil, err
	}
	return parsePosition(positionID, clientID, assetID, amount, unitPrice, totalValue,
		collateralValue, judiciaryCollateralValue, createdAt, updatedAt, purchasedAt, rowVersion)
}

func collectPositions(rows pgx.Rows) ([]*domain.Position, error) {
	var positions []*domain.Position
	for rows.Next() {
		pos, err := scanPosition(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning position row: %w", err)
		}
		positions = append(positions, pos)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating position rows: %w", err)
	}
	return positions, nil
}

func parsePosition(
	positionID, clientID, assetID uuid.UUID,
	amount, unitPrice, totalValue, collateralValue, judiciaryCollateralValue string,
	createdAt, updatedAt, purchasedAt time.Time,
	rowVersion int,
) (*domain.Position, error) {
	amountDec, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, fmt.Errorf("parsing amount %q: %w", amount, err)
	}
	unitPriceDec, err := decimal.NewFromString(unitPrice)
	if err != nil {
		return nil, fmt.Errorf("parsing unit_price %q: %w", unitPrice, err)
	}
	totalValueDec, err := decimal.NewFromString(totalValue)
	if err != nil {
		return nil, fmt.Errorf("parsing total_value %q: %w", totalValue, err)
	}
	collateralDec, err := decimal.NewFromString(collateralValue)
	if err != nil {
		return nil, fmt.Errorf("parsing collateral_value %q: %w", collateralValue, err)
	}
	judiciaryDec, err := decimal.NewFromString(judiciaryCollateralValue)
	if err != nil {
		return nil, fmt.Errorf("parsing judiciary_collateral_value %q: %w", judiciaryCollateralValue, err)
	}
	return domain.ReconstructPosition(
		positionID, clientID, assetID,
		amountDec, unitPriceDec, totalValueDec, collateralDec, judiciaryDec,
		createdAt, updatedAt, purchasedAt, rowVersion,
	)
}
