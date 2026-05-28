package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"
	// Embed the IANA timezone DB so `TZ=Asia/Jakarta` resolves even on a minimal
	// base image (distroless has no /usr/share/zoneinfo). The "today" boundary in
	// GetTodaySnapshot relies on time.Local being the shop's zone.
	_ "time/tzdata"

	"connectrpc.com/connect"

	"github.com/apotech/backend/gen/analytics_iface/v1/analyticsifacev1connect"
	"github.com/apotech/backend/gen/backup_iface/v1/backupifacev1connect"
	"github.com/apotech/backend/gen/bpjs_iface/v1/bpjsifacev1connect"
	"github.com/apotech/backend/gen/branch_iface/v1/branchifacev1connect"
	"github.com/apotech/backend/gen/customer_iface/v1/customerifacev1connect"
	"github.com/apotech/backend/gen/health_iface/v1/healthifacev1connect"
	"github.com/apotech/backend/gen/inventory_iface/v1/inventoryifacev1connect"
	"github.com/apotech/backend/gen/pos_iface/v1/posifacev1connect"
	"github.com/apotech/backend/gen/prescription_iface/v1/prescriptionifacev1connect"
	"github.com/apotech/backend/gen/purchasing_iface/v1/purchasingifacev1connect"
	"github.com/apotech/backend/gen/settings_iface/v1/settingsifacev1connect"
	"github.com/apotech/backend/gen/stocktake_iface/v1/stocktakeifacev1connect"
	"github.com/apotech/backend/gen/tax_iface/v1/taxifacev1connect"
	"github.com/apotech/backend/gen/user_iface/v1/userifacev1connect"
	"github.com/apotech/backend/gen/warehouse_iface/v1/warehouseifacev1connect"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/config"
	"github.com/apotech/backend/internal/db"
	"github.com/apotech/backend/internal/dbmigrate"
	"github.com/apotech/backend/internal/service"
	"github.com/apotech/backend/internal/web"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.MustLoad()
	gormDB := db.MustOpen(cfg)

	// Auto-migrate on boot (default on) so a freshly deployed binary brings its
	// own schema up to date — no separate migrate step in Docker / Windows.
	if cfg.Database.ShouldAutoMigrate() {
		sqlDB, err := gormDB.DB()
		if err != nil {
			log.Fatalf("get sql.DB for migrate: %v", err)
		}
		if err := dbmigrate.Run(sqlDB); err != nil {
			log.Fatalf("auto-migrate: %v", err)
		}
		slog.Info("migrations applied")
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
	slog.Info("auth policy built", "procedures", len(policy))
	interceptors := connect.WithInterceptors(
		auth.NewInterceptor(issuer, policy),
		auth.NewAuditInterceptor(gormDB),
	)

	loginLimiter := auth.NewLoginLimiter(5, 60*time.Second)
	userSvc := service.NewUsers(gormDB)
	authSvc := service.NewAuth(gormDB, issuer, refreshIssuer, loginLimiter)
	healthSvc := service.NewHealth(gormDB)
	supplierSvc := service.NewSuppliers(gormDB)
	medicineSvc := service.NewMedicines(gormDB)
	batchSvc := service.NewBatches(gormDB)
	stockSvc := service.NewStock(gormDB)
	customerSvc := service.NewCustomers(gormDB)
	saleSvc := service.NewSales(gormDB, cfg.Printer)
	analyticsSvc := service.NewAnalytics(gormDB)
	purchaseOrdersSvc := service.NewPurchaseOrders(gormDB)
	purchaseReceiptsSvc := service.NewPurchaseReceipts(gormDB)
	purchasePaymentsSvc := service.NewPurchasePayments(gormDB)
	prescriptionsSvc := service.NewPrescriptions(gormDB)
	taxInvoicesSvc := service.NewTaxInvoices(gormDB)
	bpjsClaimsSvc := service.NewBpjsClaims(gormDB)
	branchesSvc := service.NewBranches(gormDB)
	stocktakesSvc := service.NewStocktakes(gormDB)
	warehousesSvc := service.NewWarehouses(gormDB)
	transfersSvc := service.NewTransfers(gormDB)
	settingsSvc := service.NewSettings(gormDB)
	backupSvc := service.NewBackups(gormDB, cfg)

	if err := userSvc.EnsureBootstrapOwner(context.Background(), cfg.Bootstrap); err != nil {
		log.Fatalf("bootstrap: %v", err) // intentionally fatal — server can't start
	}

	// Background sweeper: hard-delete abandoned DRAFT carts the POS client missed
	// (crashes, lost sessions). In-process, single-node — like the rate limiter.
	service.StartDraftSweeper(gormDB)

	// Connect RPC handlers live on apiMux and are mounted under /api so the
	// embedded SPA can share the same origin (frontend transport baseUrl="/api").
	apiMux := http.NewServeMux()
	apiMux.Handle(healthifacev1connect.NewHealthServiceHandler(healthSvc, interceptors))
	apiMux.Handle(userifacev1connect.NewAuthServiceHandler(authSvc, interceptors))
	apiMux.Handle(userifacev1connect.NewUserServiceHandler(userSvc, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewSupplierServiceHandler(supplierSvc, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewMedicineServiceHandler(medicineSvc, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewBatchServiceHandler(batchSvc, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewStockMovementServiceHandler(stockSvc, interceptors))
	apiMux.Handle(customerifacev1connect.NewCustomerServiceHandler(customerSvc, interceptors))
	apiMux.Handle(posifacev1connect.NewSaleServiceHandler(saleSvc, interceptors))
	apiMux.Handle(analyticsifacev1connect.NewAnalyticsServiceHandler(analyticsSvc, interceptors))
	apiMux.Handle(purchasingifacev1connect.NewPurchaseOrderServiceHandler(purchaseOrdersSvc, interceptors))
	apiMux.Handle(purchasingifacev1connect.NewPurchaseReceiptServiceHandler(purchaseReceiptsSvc, interceptors))
	apiMux.Handle(purchasingifacev1connect.NewPurchasePaymentServiceHandler(purchasePaymentsSvc, interceptors))
	apiMux.Handle(prescriptionifacev1connect.NewPrescriptionServiceHandler(prescriptionsSvc, interceptors))
	apiMux.Handle(taxifacev1connect.NewTaxInvoiceServiceHandler(taxInvoicesSvc, interceptors))
	apiMux.Handle(bpjsifacev1connect.NewBpjsClaimServiceHandler(bpjsClaimsSvc, interceptors))
	apiMux.Handle(branchifacev1connect.NewBranchServiceHandler(branchesSvc, interceptors))
	apiMux.Handle(stocktakeifacev1connect.NewStocktakeServiceHandler(stocktakesSvc, interceptors))
	apiMux.Handle(warehouseifacev1connect.NewWarehouseServiceHandler(warehousesSvc, interceptors))
	apiMux.Handle(warehouseifacev1connect.NewStockTransferServiceHandler(transfersSvc, interceptors))
	apiMux.Handle(settingsifacev1connect.NewSettingsServiceHandler(settingsSvc, interceptors))
	apiMux.Handle(backupifacev1connect.NewBackupServiceHandler(backupSvc, interceptors))

	// Root mux: /api/* → Connect handlers, /healthz → liveness probe,
	// everything else → the embedded SPA (single self-contained binary).
	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", apiMux))
	root.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	root.Handle("/", web.Handler())

	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true) // h2c: HTTP/2 over plain TCP for gRPC/Connect streams

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:      addr,
		Handler:   root,
		Protocols: &protocols,
	}

	slog.Info("apotech listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
