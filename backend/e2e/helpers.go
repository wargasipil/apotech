package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/apotech/backend/gen/analytics_iface/v1/analyticsifacev1connect"
	"github.com/apotech/backend/gen/backup_iface/v1/backupifacev1connect"
	"github.com/apotech/backend/gen/customer_iface/v1/customerifacev1connect"
	"github.com/apotech/backend/gen/inventory_iface/v1/inventoryifacev1connect"
	posifacev1connect "github.com/apotech/backend/gen/pos_iface/v1/posifacev1connect"
	"github.com/apotech/backend/gen/purchasing_iface/v1/purchasingifacev1connect"
	"github.com/apotech/backend/gen/settings_iface/v1/settingsifacev1connect"
	"github.com/apotech/backend/gen/stocktake_iface/v1/stocktakeifacev1connect"
	"github.com/apotech/backend/gen/unit_iface/v1/unitifacev1connect"
	userifacev1 "github.com/apotech/backend/gen/user_iface/v1"
	"github.com/apotech/backend/gen/user_iface/v1/userifacev1connect"
	"github.com/apotech/backend/gen/warehouse_iface/v1/warehouseifacev1connect"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/config"
	"github.com/apotech/backend/internal/db"
	"github.com/apotech/backend/internal/dbmigrate"
	"github.com/apotech/backend/internal/service"
)

// TestUser holds the credentials of a user known to the test environment.
type TestUser struct {
	Email    string
	Password string
}

// Env is the in-process test environment: a real Postgres-backed handler
// stack served by httptest, plus typed Connect clients for the auth + user
// services.
type Env struct {
	Server    *httptest.Server
	DB        *gorm.DB
	Auth      userifacev1connect.AuthServiceClient
	Users     userifacev1connect.UserServiceClient
	Customers customerifacev1connect.CustomerServiceClient
	Suppliers  inventoryifacev1connect.SupplierServiceClient
	Medicines  inventoryifacev1connect.MedicineServiceClient
	Batches    inventoryifacev1connect.BatchServiceClient
	Stock      inventoryifacev1connect.StockMovementServiceClient
	Sales      posifacev1connect.SaleServiceClient
	Stocktakes stocktakeifacev1connect.StocktakeServiceClient
	Warehouses warehouseifacev1connect.WarehouseServiceClient
	Transfers  warehouseifacev1connect.StockTransferServiceClient
	POs        purchasingifacev1connect.PurchaseOrderServiceClient
	Receipts   purchasingifacev1connect.PurchaseReceiptServiceClient
	Settings   settingsifacev1connect.SettingsServiceClient
	Units      unitifacev1connect.UnitServiceClient
	Backups    backupifacev1connect.BackupServiceClient
	Analytics  analyticsifacev1connect.AnalyticsServiceClient
	// BackupDir is t.TempDir() — Setup wires Backups against this directory via
	// NewBackupsWithDir so tests can't write to the real ./backups.
	BackupDir string
	// Cfg is the loaded config — needed by tests that construct a service
	// standalone (e.g. TestBackup_CleanupOnPgDumpFailure clones Database with
	// bad credentials).
	Cfg   *config.Config
	Owner TestUser
}

// AuthHeader returns "Bearer <access_token>" after logging in the owner.
// Convenience for tests that need an authenticated call.
func (e *Env) AuthHeader(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	res, err := e.Auth.Login(ctx, connect.NewRequest(&userifacev1.LoginRequest{
		Email:    e.Owner.Email,
		Password: e.Owner.Password,
	}))
	if err != nil {
		t.Fatalf("owner login: %v", err)
	}
	return "Bearer " + res.Msg.AccessToken
}

// SetupEnv builds the same handler stack cmd/server/main.go uses, wraps it
// in an httptest.Server, and ensures the bootstrap-owner user exists with
// the password from config.yaml.
//
// Side effect: every call upserts the bootstrap-owner password back to what
// config.yaml says. Documented in CLAUDE.md.
func SetupEnv(t *testing.T) *Env {
	t.Helper()

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	// applyTestDBOverride is a no-op under the default (Postgres) build and
	// rewrites Database.Driver/Path to a fresh per-test SQLite file under
	// `-tags sqlite`.
	applyTestDBOverride(t, cfg)

	gormDB, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Close this test's connection pool when the test ends — otherwise each
	// SetupEnv leaks a pool and the suite accumulates connections until Postgres
	// max_connections is exhausted (flaky "can't get a connection" failures on
	// random tests under load). Registered before the srv cleanup below so it
	// runs after it (t.Cleanup is LIFO: stop serving, then close the DB). Bound
	// the pool too as cheap insurance against momentary overlap with the dev
	// backend / make web; tests are sequential + unary, so 10 is ample.
	if sqlDB, derr := gormDB.DB(); derr == nil {
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetConnMaxLifetime(time.Minute)
		t.Cleanup(func() { _ = sqlDB.Close() })

		// SQLite tests start from an empty file → run migrations here.
		// Postgres tests use the pre-migrated dev DB → skip (no-op).
		if runMigrationsOnSetup {
			if err := dbmigrate.Run(sqlDB); err != nil {
				t.Fatalf("dbmigrate: %v", err)
			}
		}
	}

	issuer := &auth.Issuer{
		Secret: []byte(cfg.Auth.JWTSecret),
		TTL:    cfg.Auth.AccessTokenTTL,
	}
	refreshIssuer := &auth.RefreshIssuer{
		DB:  gormDB,
		TTL: cfg.Auth.RefreshTokenTTL,
	}

	policy := auth.BuildPolicy()
	interceptors := connect.WithInterceptors(auth.NewInterceptor(issuer, policy))

	// Tests use a generous limiter so concurrent test runs don't trip it.
	loginLimiter := auth.NewLoginLimiter(1000, time.Second)
	authSvc := service.NewAuth(gormDB, issuer, refreshIssuer, loginLimiter)
	userSvc := service.NewUsers(gormDB)
	customerSvc := service.NewCustomers(gormDB)
	supplierSvc := service.NewSuppliers(gormDB)
	medicineSvc := service.NewMedicines(gormDB)
	batchSvc := service.NewBatches(gormDB)
	stockSvc := service.NewStock(gormDB)
	saleSvc := service.NewSales(gormDB, cfg.Printer)
	stocktakeSvc := service.NewStocktakes(gormDB)
	warehouseSvc := service.NewWarehouses(gormDB)
	transferSvc := service.NewTransfers(gormDB)
	poSvc := service.NewPurchaseOrders(gormDB)
	receiptSvc := service.NewPurchaseReceipts(gormDB)
	settingsSvc := service.NewSettings(gormDB)
	unitsSvc := service.NewUnits(gormDB)
	// Wire BackupService against a per-test temp dir so backup_<ts>/ never
	// pollutes the real ./backups; the test can read files under backupDir.
	backupDir := t.TempDir()
	backupSvc := service.NewBackupsWithDir(gormDB, cfg, backupDir)
	analyticsSvc := service.NewAnalytics(gormDB)

	if cfg.Bootstrap.OwnerEmail == "" {
		t.Fatalf("config.bootstrap.owner_email is empty; set it in config.yaml so tests have a known user")
	}
	if err := userSvc.EnsureBootstrapOwner(context.Background(), cfg.Bootstrap); err != nil {
		t.Fatalf("ensure bootstrap owner: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(userifacev1connect.NewAuthServiceHandler(authSvc, interceptors))
	mux.Handle(userifacev1connect.NewUserServiceHandler(userSvc, interceptors))
	mux.Handle(customerifacev1connect.NewCustomerServiceHandler(customerSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewSupplierServiceHandler(supplierSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewMedicineServiceHandler(medicineSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewBatchServiceHandler(batchSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewStockMovementServiceHandler(stockSvc, interceptors))
	mux.Handle(posifacev1connect.NewSaleServiceHandler(saleSvc, interceptors))
	mux.Handle(stocktakeifacev1connect.NewStocktakeServiceHandler(stocktakeSvc, interceptors))
	mux.Handle(warehouseifacev1connect.NewWarehouseServiceHandler(warehouseSvc, interceptors))
	mux.Handle(warehouseifacev1connect.NewStockTransferServiceHandler(transferSvc, interceptors))
	mux.Handle(purchasingifacev1connect.NewPurchaseOrderServiceHandler(poSvc, interceptors))
	mux.Handle(purchasingifacev1connect.NewPurchaseReceiptServiceHandler(receiptSvc, interceptors))
	mux.Handle(settingsifacev1connect.NewSettingsServiceHandler(settingsSvc, interceptors))
	mux.Handle(unitifacev1connect.NewUnitServiceHandler(unitsSvc, interceptors))
	mux.Handle(backupifacev1connect.NewBackupServiceHandler(backupSvc, interceptors))
	mux.Handle(analyticsifacev1connect.NewAnalyticsServiceHandler(analyticsSvc, interceptors))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &Env{
		Server:    srv,
		DB:        gormDB,
		Auth:      userifacev1connect.NewAuthServiceClient(srv.Client(), srv.URL),
		Users:     userifacev1connect.NewUserServiceClient(srv.Client(), srv.URL),
		Customers: customerifacev1connect.NewCustomerServiceClient(srv.Client(), srv.URL),
		Suppliers:  inventoryifacev1connect.NewSupplierServiceClient(srv.Client(), srv.URL),
		Medicines:  inventoryifacev1connect.NewMedicineServiceClient(srv.Client(), srv.URL),
		Batches:    inventoryifacev1connect.NewBatchServiceClient(srv.Client(), srv.URL),
		Stock:      inventoryifacev1connect.NewStockMovementServiceClient(srv.Client(), srv.URL),
		Sales:      posifacev1connect.NewSaleServiceClient(srv.Client(), srv.URL),
		Stocktakes: stocktakeifacev1connect.NewStocktakeServiceClient(srv.Client(), srv.URL),
		Warehouses: warehouseifacev1connect.NewWarehouseServiceClient(srv.Client(), srv.URL),
		Transfers:  warehouseifacev1connect.NewStockTransferServiceClient(srv.Client(), srv.URL),
		POs:        purchasingifacev1connect.NewPurchaseOrderServiceClient(srv.Client(), srv.URL),
		Receipts:   purchasingifacev1connect.NewPurchaseReceiptServiceClient(srv.Client(), srv.URL),
		Settings:   settingsifacev1connect.NewSettingsServiceClient(srv.Client(), srv.URL),
		Units:      unitifacev1connect.NewUnitServiceClient(srv.Client(), srv.URL),
		Backups:    backupifacev1connect.NewBackupServiceClient(srv.Client(), srv.URL),
		Analytics:  analyticsifacev1connect.NewAnalyticsServiceClient(srv.Client(), srv.URL),
		BackupDir:  backupDir,
		Cfg:        cfg,
		Owner: TestUser{
			Email:    cfg.Bootstrap.OwnerEmail,
			Password: cfg.Bootstrap.OwnerPassword,
		},
	}
}
