package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/primolabs-org/spec-star-go/internal/domain"
	"github.com/primolabs-org/spec-star-go/internal/platform"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func parseLogEntries(buf *bytes.Buffer) []map[string]any {
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// --- Mock implementations ---

type mockClientRepository struct {
	findByIDFn func(ctx context.Context, clientID uuid.UUID) (*domain.Client, error)
}

func (m *mockClientRepository) FindByID(ctx context.Context, clientID uuid.UUID) (*domain.Client, error) {
	return m.findByIDFn(ctx, clientID)
}

func (m *mockClientRepository) Create(_ context.Context, _ *domain.Client) error {
	return nil
}

type mockAssetRepository struct {
	findByIDFn func(ctx context.Context, assetID uuid.UUID) (*domain.Asset, error)
}

func (m *mockAssetRepository) FindByID(ctx context.Context, assetID uuid.UUID) (*domain.Asset, error) {
	return m.findByIDFn(ctx, assetID)
}

func (m *mockAssetRepository) FindByInstrumentID(_ context.Context, _ string) (*domain.Asset, error) {
	return nil, nil
}

func (m *mockAssetRepository) Create(_ context.Context, _ *domain.Asset) error {
	return nil
}

type mockPositionRepository struct {
	createFn func(ctx context.Context, position *domain.Position) error
}

func (m *mockPositionRepository) FindByID(_ context.Context, _ uuid.UUID) (*domain.Position, error) {
	return nil, nil
}

func (m *mockPositionRepository) FindByClientAndAsset(_ context.Context, _, _ uuid.UUID) ([]*domain.Position, error) {
	return nil, nil
}

func (m *mockPositionRepository) FindByClientAndInstrument(_ context.Context, _ uuid.UUID, _ string) ([]*domain.Position, error) {
	return nil, nil
}

func (m *mockPositionRepository) Create(ctx context.Context, position *domain.Position) error {
	return m.createFn(ctx, position)
}

func (m *mockPositionRepository) Update(_ context.Context, _ *domain.Position) error {
	return nil
}

type mockProcessedCommandRepository struct {
	findByTypeAndOrderIDFn func(ctx context.Context, commandType, orderID string) (*domain.ProcessedCommand, error)
	createFn               func(ctx context.Context, cmd *domain.ProcessedCommand) error
}

func (m *mockProcessedCommandRepository) FindByTypeAndOrderID(ctx context.Context, commandType, orderID string) (*domain.ProcessedCommand, error) {
	return m.findByTypeAndOrderIDFn(ctx, commandType, orderID)
}

func (m *mockProcessedCommandRepository) Create(ctx context.Context, cmd *domain.ProcessedCommand) error {
	return m.createFn(ctx, cmd)
}

type mockUnitOfWork struct {
	doFn func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (m *mockUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.doFn(ctx, fn)
}

// --- Test helpers ---

func validRequest() DepositRequest {
	return DepositRequest{
		ClientID:  uuid.New().String(),
		AssetID:   uuid.New().String(),
		OrderID:   "order-123",
		Amount:    "100.50",
		UnitPrice: "10.25",
	}
}

func newTestAsset(productType domain.ProductType) *domain.Asset {
	return domain.ReconstructAsset(
		uuid.New(),
		"INST-001",
		productType,
		"offer-1",
		"entity-1",
		"doc-1",
		"MKT",
		"Test Asset",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Now(),
	)
}

func newTestClient() *domain.Client {
	client, _ := domain.NewClient("ext-001")
	return client
}

func buildService(
	clients *mockClientRepository,
	assets *mockAssetRepository,
	positions *mockPositionRepository,
	processedCmds *mockProcessedCommandRepository,
	uow *mockUnitOfWork,
) *DepositService {
	return NewDepositService(clients, assets, positions, processedCmds, uow)
}

func defaultMocks() (*mockClientRepository, *mockAssetRepository, *mockPositionRepository, *mockProcessedCommandRepository, *mockUnitOfWork) {
	clients := &mockClientRepository{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Client, error) {
			return newTestClient(), nil
		},
	}
	assets := &mockAssetRepository{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Asset, error) {
			return newTestAsset(domain.ProductTypeCDB), nil
		},
	}
	positions := &mockPositionRepository{
		createFn: func(_ context.Context, _ *domain.Position) error {
			return nil
		},
	}
	processedCmds := &mockProcessedCommandRepository{
		findByTypeAndOrderIDFn: func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
			return nil, domain.ErrNotFound
		},
		createFn: func(_ context.Context, _ *domain.ProcessedCommand) error {
			return nil
		},
	}
	uow := &mockUnitOfWork{
		doFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	}
	return clients, assets, positions, processedCmds, uow
}

// --- Happy path tests ---

func TestExecute_ValidDeposit_Returns201(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, status)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.ClientID != req.ClientID {
		t.Errorf("expected client_id %s, got %s", req.ClientID, resp.ClientID)
	}
	if resp.AssetID != req.AssetID {
		t.Errorf("expected asset_id %s, got %s", req.AssetID, resp.AssetID)
	}
}

func TestExecute_ValidDeposit_ResponseFieldsMatchPosition(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	resp, _, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := uuid.Parse(resp.PositionID); err != nil {
		t.Errorf("position_id is not a valid UUID: %s", resp.PositionID)
	}

	amount, _ := decimal.NewFromString(req.Amount)
	unitPrice, _ := decimal.NewFromString(req.UnitPrice)
	expectedTotal := amount.Mul(unitPrice)

	if resp.Amount != amount.String() {
		t.Errorf("expected amount %s, got %s", amount.String(), resp.Amount)
	}
	if resp.UnitPrice != unitPrice.String() {
		t.Errorf("expected unit_price %s, got %s", unitPrice.String(), resp.UnitPrice)
	}
	if resp.TotalValue != expectedTotal.String() {
		t.Errorf("expected total_value %s, got %s", expectedTotal.String(), resp.TotalValue)
	}
	if resp.CollateralValue != "0" {
		t.Errorf("expected collateral_value 0, got %s", resp.CollateralValue)
	}
	if resp.JudiciaryCollateralValue != "0" {
		t.Errorf("expected judiciary_collateral_value 0, got %s", resp.JudiciaryCollateralValue)
	}
	if _, err := time.Parse(time.RFC3339, resp.PurchasedAt); err != nil {
		t.Errorf("purchased_at is not valid RFC3339: %s", resp.PurchasedAt)
	}
	if _, err := time.Parse(time.RFC3339, resp.CreatedAt); err != nil {
		t.Errorf("created_at is not valid RFC3339: %s", resp.CreatedAt)
	}
	if _, err := time.Parse(time.RFC3339, resp.UpdatedAt); err != nil {
		t.Errorf("updated_at is not valid RFC3339: %s", resp.UpdatedAt)
	}
}

// --- Idempotency replay tests ---

func TestExecute_IdempotentReplay_Returns200(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()

	snapshot := DepositResponse{
		PositionID:               uuid.New().String(),
		ClientID:                 uuid.New().String(),
		AssetID:                  uuid.New().String(),
		Amount:                   "100",
		UnitPrice:                "10",
		TotalValue:               "1000",
		CollateralValue:          "0",
		JudiciaryCollateralValue: "0",
		PurchasedAt:              time.Now().UTC().Format(time.RFC3339),
		CreatedAt:                time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:                time.Now().UTC().Format(time.RFC3339),
	}
	snapshotBytes, _ := json.Marshal(snapshot)

	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		return domain.ReconstructProcessedCommand(
			uuid.New(), "DEPOSIT", "order-123", uuid.New(), snapshotBytes, time.Now(),
		), nil
	}

	createCalled := false
	positions.createFn = func(_ context.Context, _ *domain.Position) error {
		createCalled = true
		return nil
	}
	processedCmds.createFn = func(_ context.Context, _ *domain.ProcessedCommand) error {
		createCalled = true
		return nil
	}

	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.OrderID = "order-123"

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if resp.PositionID != snapshot.PositionID {
		t.Errorf("expected position_id %s, got %s", snapshot.PositionID, resp.PositionID)
	}
	if createCalled {
		t.Error("expected no Create calls for idempotent replay")
	}
}

// --- Idempotency concurrent race tests ---

func TestExecute_RaceCondition_ReplayReturns200(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()

	snapshot := DepositResponse{
		PositionID: uuid.New().String(),
		ClientID:   uuid.New().String(),
		AssetID:    uuid.New().String(),
		Amount:     "50",
		UnitPrice:  "5",
		TotalValue: "250",
	}
	snapshotBytes, _ := json.Marshal(snapshot)

	callCount := 0
	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		callCount++
		if callCount == 1 {
			return nil, domain.ErrNotFound
		}
		return domain.ReconstructProcessedCommand(
			uuid.New(), "DEPOSIT", "order-123", uuid.New(), snapshotBytes, time.Now(),
		), nil
	}

	processedCmds.createFn = func(_ context.Context, _ *domain.ProcessedCommand) error {
		return domain.ErrDuplicate
	}

	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	resp, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if resp.PositionID != snapshot.PositionID {
		t.Errorf("expected position_id %s, got %s", snapshot.PositionID, resp.PositionID)
	}
}

func TestExecute_RaceCondition_RereadFails_Returns409(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()

	callCount := 0
	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		callCount++
		if callCount == 1 {
			return nil, domain.ErrNotFound
		}
		return nil, errors.New("database gone")
	}

	processedCmds.createFn = func(_ context.Context, _ *domain.ProcessedCommand) error {
		return domain.ErrDuplicate
	}

	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if err == nil {
		t.Fatal("expected error")
	}
	if status != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, status)
	}
}

// --- Validation failure tests ---

func TestExecute_MissingClientID_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.ClientID = ""

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "client_id is required" {
		t.Fatalf("expected 'client_id is required', got %v", err)
	}
}

func TestExecute_InvalidClientID_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.ClientID = "not-a-uuid"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "invalid client_id" {
		t.Fatalf("expected 'invalid client_id', got %v", err)
	}
}

func TestExecute_MissingAssetID_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.AssetID = ""

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "asset_id is required" {
		t.Fatalf("expected 'asset_id is required', got %v", err)
	}
}

func TestExecute_InvalidAssetID_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.AssetID = "not-a-uuid"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "invalid asset_id" {
		t.Fatalf("expected 'invalid asset_id', got %v", err)
	}
}

func TestExecute_MissingOrderID_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.OrderID = ""

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "order_id is required" {
		t.Fatalf("expected 'order_id is required', got %v", err)
	}
}

func TestExecute_MissingAmount_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.Amount = ""

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "amount is required" {
		t.Fatalf("expected 'amount is required', got %v", err)
	}
}

func TestExecute_InvalidAmount_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.Amount = "abc"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "invalid amount" {
		t.Fatalf("expected 'invalid amount', got %v", err)
	}
}

func TestExecute_MissingUnitPrice_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.UnitPrice = ""

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "unit_price is required" {
		t.Fatalf("expected 'unit_price is required', got %v", err)
	}
}

func TestExecute_InvalidUnitPrice_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.UnitPrice = "xyz"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "invalid unit_price" {
		t.Fatalf("expected 'invalid unit_price', got %v", err)
	}
}

func TestExecute_ZeroAmount_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.Amount = "0"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "amount must be positive" {
		t.Fatalf("expected 'amount must be positive', got %v", err)
	}
}

func TestExecute_NegativeAmount_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.Amount = "-5"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "amount must be positive" {
		t.Fatalf("expected 'amount must be positive', got %v", err)
	}
}

func TestExecute_ZeroUnitPrice_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.UnitPrice = "0"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "unit_price must be positive" {
		t.Fatalf("expected 'unit_price must be positive', got %v", err)
	}
}

func TestExecute_NegativeUnitPrice_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.UnitPrice = "-1"

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "unit_price must be positive" {
		t.Fatalf("expected 'unit_price must be positive', got %v", err)
	}
}

// --- Entity lookup failure tests ---

func TestExecute_ClientNotFound_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	clients.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Client, error) {
		return nil, domain.ErrNotFound
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "client not found" {
		t.Fatalf("expected 'client not found', got %v", err)
	}
}

func TestExecute_AssetNotFound_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	assets.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Asset, error) {
		return nil, domain.ErrNotFound
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "asset not found" {
		t.Fatalf("expected 'asset not found', got %v", err)
	}
}

func TestExecute_UnsupportedProductType_Returns422(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	assets.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Asset, error) {
		return domain.ReconstructAsset(
			uuid.New(), "INST-001", domain.ProductType("INVALID"), "", "", "", "", "Bad Asset",
			time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Now(),
		), nil
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if err == nil || err.Error() != "unsupported product type" {
		t.Fatalf("expected 'unsupported product type', got %v", err)
	}
}

// --- Unexpected error tests ---

func TestExecute_ClientFindUnexpectedError_Returns500(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	clients.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Client, error) {
		return nil, errors.New("connection refused")
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExecute_AssetFindUnexpectedError_Returns500(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	assets.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Asset, error) {
		return nil, errors.New("timeout")
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExecute_ProcessedCommandFindUnexpectedError_Returns500(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		return nil, errors.New("database crashed")
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExecute_PositionCreateUnexpectedError_Returns500(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	positions.createFn = func(_ context.Context, _ *domain.Position) error {
		return errors.New("disk full")
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExecute_NilUUIDClientID_NewProcessedCommandFails_Returns500(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.ClientID = uuid.Nil.String()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExecute_IdempotentReplay_CorruptSnapshot_Returns500(t *testing.T) {
	clients, assets, positions, processedCmds, uow := defaultMocks()

	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		return domain.ReconstructProcessedCommand(
			uuid.New(), "DEPOSIT", "order-123", uuid.New(), []byte("not-json"), time.Now(),
		), nil
	}

	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Logging tests ---

func TestExecute_ProcessedCommandInfraFailure_LogsError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := platform.WithLogger(context.Background(), logger)

	clients, assets, positions, processedCmds, uow := defaultMocks()
	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		return nil, errors.New("database crashed")
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, _ := svc.Execute(ctx, req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}

	entries := parseLogEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry["level"] != "ERROR" {
		t.Errorf("expected level ERROR, got %v", entry["level"])
	}
	if entry["outcome"] != "failed" {
		t.Errorf("expected outcome failed, got %v", entry["outcome"])
	}
	if entry["order_id"] != req.OrderID {
		t.Errorf("expected order_id %s, got %v", req.OrderID, entry["order_id"])
	}
	if _, ok := entry["error"]; !ok {
		t.Error("expected error field in log entry")
	}
}

func TestExecute_IdempotencyReplay_LogsInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := platform.WithLogger(context.Background(), logger)

	clients, assets, positions, processedCmds, uow := defaultMocks()

	snapshot := DepositResponse{
		PositionID: uuid.New().String(),
		ClientID:   uuid.New().String(),
		AssetID:    uuid.New().String(),
		Amount:     "100", UnitPrice: "10", TotalValue: "1000",
		CollateralValue: "0", JudiciaryCollateralValue: "0",
		PurchasedAt: time.Now().UTC().Format(time.RFC3339),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	snapshotBytes, _ := json.Marshal(snapshot)

	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		return domain.ReconstructProcessedCommand(
			uuid.New(), "DEPOSIT", "order-123", uuid.New(), snapshotBytes, time.Now(),
		), nil
	}

	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.OrderID = "order-123"

	_, status, err := svc.Execute(ctx, req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	entries := parseLogEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry["level"] != "INFO" {
		t.Errorf("expected level INFO, got %v", entry["level"])
	}
	if entry["outcome"] != "replayed" {
		t.Errorf("expected outcome replayed, got %v", entry["outcome"])
	}
	if entry["order_id"] != "order-123" {
		t.Errorf("expected order_id order-123, got %v", entry["order_id"])
	}
}

func TestExecute_RaceConditionReplay_LogsInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := platform.WithLogger(context.Background(), logger)

	clients, assets, positions, processedCmds, uow := defaultMocks()

	snapshot := DepositResponse{
		PositionID: uuid.New().String(),
		ClientID:   uuid.New().String(),
		AssetID:    uuid.New().String(),
		Amount:     "50", UnitPrice: "5", TotalValue: "250",
	}
	snapshotBytes, _ := json.Marshal(snapshot)

	callCount := 0
	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		callCount++
		if callCount == 1 {
			return nil, domain.ErrNotFound
		}
		return domain.ReconstructProcessedCommand(
			uuid.New(), "DEPOSIT", "order-race", uuid.New(), snapshotBytes, time.Now(),
		), nil
	}

	processedCmds.createFn = func(_ context.Context, _ *domain.ProcessedCommand) error {
		return domain.ErrDuplicate
	}

	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.OrderID = "order-race"

	_, status, err := svc.Execute(ctx, req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	entries := parseLogEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry["level"] != "INFO" {
		t.Errorf("expected level INFO, got %v", entry["level"])
	}
	if entry["outcome"] != "replayed" {
		t.Errorf("expected outcome replayed, got %v", entry["outcome"])
	}
	if entry["order_id"] != "order-race" {
		t.Errorf("expected order_id order-race, got %v", entry["order_id"])
	}
}

func TestExecute_UnitOfWorkInfraFailure_LogsError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := platform.WithLogger(context.Background(), logger)

	clients, assets, positions, processedCmds, uow := defaultMocks()
	positions.createFn = func(_ context.Context, _ *domain.Position) error {
		return errors.New("disk full")
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, _ := svc.Execute(ctx, req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}

	entries := parseLogEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry["level"] != "ERROR" {
		t.Errorf("expected level ERROR, got %v", entry["level"])
	}
	if entry["outcome"] != "failed" {
		t.Errorf("expected outcome failed, got %v", entry["outcome"])
	}
	if entry["order_id"] != req.OrderID {
		t.Errorf("expected order_id %s, got %v", req.OrderID, entry["order_id"])
	}
	if _, ok := entry["error"]; !ok {
		t.Error("expected error field in log entry")
	}
}

func TestExecute_ClientInfraFailure_LogsError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := platform.WithLogger(context.Background(), logger)

	clients, assets, positions, processedCmds, uow := defaultMocks()
	clients.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Client, error) {
		return nil, errors.New("connection refused")
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, _ := svc.Execute(ctx, req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}

	entries := parseLogEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry["level"] != "ERROR" {
		t.Errorf("expected level ERROR, got %v", entry["level"])
	}
	if entry["outcome"] != "failed" {
		t.Errorf("expected outcome failed, got %v", entry["outcome"])
	}
	if entry["client_id"] != req.ClientID {
		t.Errorf("expected client_id %s, got %v", req.ClientID, entry["client_id"])
	}
	if entry["order_id"] != req.OrderID {
		t.Errorf("expected order_id %s, got %v", req.OrderID, entry["order_id"])
	}
	if _, ok := entry["error"]; !ok {
		t.Error("expected error field in log entry")
	}
}

func TestExecute_AssetInfraFailure_LogsError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := platform.WithLogger(context.Background(), logger)

	clients, assets, positions, processedCmds, uow := defaultMocks()
	assets.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Asset, error) {
		return nil, errors.New("timeout")
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, _ := svc.Execute(ctx, req)

	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}

	entries := parseLogEntries(&buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry["level"] != "ERROR" {
		t.Errorf("expected level ERROR, got %v", entry["level"])
	}
	if entry["outcome"] != "failed" {
		t.Errorf("expected outcome failed, got %v", entry["outcome"])
	}
	if entry["asset_id"] != req.AssetID {
		t.Errorf("expected asset_id %s, got %v", req.AssetID, entry["asset_id"])
	}
	if entry["order_id"] != req.OrderID {
		t.Errorf("expected order_id %s, got %v", req.OrderID, entry["order_id"])
	}
	if _, ok := entry["error"]; !ok {
		t.Error("expected error field in log entry")
	}
}

func TestExecute_ClientNotFound_NoLogEmitted(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := platform.WithLogger(context.Background(), logger)

	clients, assets, positions, processedCmds, uow := defaultMocks()
	clients.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Client, error) {
		return nil, domain.ErrNotFound
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, _ := svc.Execute(ctx, req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no log output for 4xx path, got: %s", buf.String())
	}
}

func TestExecute_AssetNotFound_NoLogEmitted(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := platform.WithLogger(context.Background(), logger)

	clients, assets, positions, processedCmds, uow := defaultMocks()
	assets.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Asset, error) {
		return nil, domain.ErrNotFound
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, _ := svc.Execute(ctx, req)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, status)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no log output for 4xx path, got: %s", buf.String())
	}
}

// --- Tracing test helpers ---

func setupTestTracing(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	})
	return sr
}

func findSpanAttribute(span sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			return attr.Value, true
		}
	}
	return attribute.Value{}, false
}

// --- Deposit tracing tests ---

func TestExecute_ValidDeposit_CreatesSpanWithOutcomeSuccess(t *testing.T) {
	sr := setupTestTracing(t)
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, status)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name() != "deposit.execute" {
		t.Errorf("expected span name deposit.execute, got %s", span.Name())
	}
	if span.Status().Code != codes.Ok {
		t.Errorf("expected span status OK, got %v", span.Status().Code)
	}
	outcome, ok := findSpanAttribute(span, "wallet.outcome")
	if !ok {
		t.Fatal("expected wallet.outcome attribute")
	}
	if outcome.AsString() != "success" {
		t.Errorf("expected wallet.outcome=success, got %s", outcome.AsString())
	}
	orderID, ok := findSpanAttribute(span, "wallet.order_id")
	if !ok {
		t.Fatal("expected wallet.order_id attribute")
	}
	if orderID.AsString() != req.OrderID {
		t.Errorf("expected wallet.order_id=%s, got %s", req.OrderID, orderID.AsString())
	}
}

func TestExecute_IdempotentReplay_SpanOutcomeReplayed(t *testing.T) {
	sr := setupTestTracing(t)
	clients, assets, positions, processedCmds, uow := defaultMocks()

	snapshot := DepositResponse{
		PositionID: uuid.New().String(), ClientID: uuid.New().String(),
		AssetID: uuid.New().String(), Amount: "100", UnitPrice: "10",
		TotalValue: "1000", CollateralValue: "0", JudiciaryCollateralValue: "0",
		PurchasedAt: time.Now().UTC().Format(time.RFC3339),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	snapshotBytes, _ := json.Marshal(snapshot)

	processedCmds.findByTypeAndOrderIDFn = func(_ context.Context, _, _ string) (*domain.ProcessedCommand, error) {
		return domain.ReconstructProcessedCommand(
			uuid.New(), "DEPOSIT", "order-123", uuid.New(), snapshotBytes, time.Now(),
		), nil
	}

	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, _, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status().Code != codes.Ok {
		t.Errorf("expected span status OK, got %v", span.Status().Code)
	}
	outcome, ok := findSpanAttribute(span, "wallet.outcome")
	if !ok {
		t.Fatal("expected wallet.outcome attribute")
	}
	if outcome.AsString() != "replayed" {
		t.Errorf("expected wallet.outcome=replayed, got %s", outcome.AsString())
	}
}

func TestExecute_ValidationFailure_SpanStatusError(t *testing.T) {
	sr := setupTestTracing(t)
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()
	req.ClientID = ""

	_, _, err := svc.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status().Code != codes.Error {
		t.Errorf("expected span status Error, got %v", span.Status().Code)
	}
	outcome, ok := findSpanAttribute(span, "wallet.outcome")
	if !ok {
		t.Fatal("expected wallet.outcome attribute")
	}
	if outcome.AsString() != "failed" {
		t.Errorf("expected wallet.outcome=failed, got %s", outcome.AsString())
	}
}

func TestExecute_InfrastructureError_SpanStatusError(t *testing.T) {
	sr := setupTestTracing(t)
	clients, assets, positions, processedCmds, uow := defaultMocks()
	clients.findByIDFn = func(_ context.Context, _ uuid.UUID) (*domain.Client, error) {
		return nil, errors.New("connection refused")
	}
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, _, err := svc.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status().Code != codes.Error {
		t.Errorf("expected span status Error, got %v", span.Status().Code)
	}
	outcome, ok := findSpanAttribute(span, "wallet.outcome")
	if !ok {
		t.Fatal("expected wallet.outcome attribute")
	}
	if outcome.AsString() != "failed" {
		t.Errorf("expected wallet.outcome=failed, got %s", outcome.AsString())
	}
	orderID, ok := findSpanAttribute(span, "wallet.order_id")
	if !ok {
		t.Fatal("expected wallet.order_id attribute")
	}
	if orderID.AsString() != req.OrderID {
		t.Errorf("expected wallet.order_id=%s, got %s", req.OrderID, orderID.AsString())
	}
}

func TestExecute_MarshalSnapshotError_SpanStatusError(t *testing.T) {
	original := depositMarshalJSON
	depositMarshalJSON = func(v any) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}
	t.Cleanup(func() { depositMarshalJSON = original })

	sr := setupTestTracing(t)
	clients, assets, positions, processedCmds, uow := defaultMocks()
	svc := buildService(clients, assets, positions, processedCmds, uow)
	req := validRequest()

	_, status, err := svc.Execute(context.Background(), req)

	if err == nil {
		t.Fatal("expected error")
	}
	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status().Code != codes.Error {
		t.Errorf("expected span status Error, got %v", span.Status().Code)
	}
	outcome, ok := findSpanAttribute(span, "wallet.outcome")
	if !ok {
		t.Fatal("expected wallet.outcome attribute")
	}
	if outcome.AsString() != "failed" {
		t.Errorf("expected wallet.outcome=failed, got %s", outcome.AsString())
	}
}
