//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/primolabs-org/spec-star-go/internal/domain"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required for integration tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("creating test pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func withTestTx(t *testing.T, pool *pgxpool.Pool) context.Context {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("beginning test transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return context.WithValue(ctx, txKey{}, tx)
}

func createTestClient(t *testing.T, ctx context.Context, repo *ClientRepository) *domain.Client {
	t.Helper()
	client, err := domain.NewClient("EXT-" + uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	if err := repo.Create(ctx, client); err != nil {
		t.Fatalf("inserting test client: %v", err)
	}
	return client
}

func createTestAsset(t *testing.T, ctx context.Context, repo *AssetRepository, instrumentID string) *domain.Asset {
	t.Helper()
	asset, err := domain.NewAsset(
		instrumentID,
		domain.ProductTypeCDB,
		"",
		"EMITTER-001",
		"",
		"",
		"Test CDB",
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("creating test asset: %v", err)
	}
	if err := repo.Create(ctx, asset); err != nil {
		t.Fatalf("inserting test asset: %v", err)
	}
	return asset
}

func timesEqualMicro(a, b time.Time) bool {
	return a.Truncate(time.Microsecond).Equal(b.Truncate(time.Microsecond))
}

func datesEqual(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
