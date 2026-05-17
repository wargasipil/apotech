package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"

	"github.com/apotech/backend/gen/analytics_iface/v1/analyticsifacev1connect"
	"github.com/apotech/backend/gen/bpjs_iface/v1/bpjsifacev1connect"
	"github.com/apotech/backend/gen/branch_iface/v1/branchifacev1connect"
	"github.com/apotech/backend/gen/customer_iface/v1/customerifacev1connect"
	"github.com/apotech/backend/gen/health_iface/v1/healthifacev1connect"
	"github.com/apotech/backend/gen/inventory_iface/v1/inventoryifacev1connect"
	"github.com/apotech/backend/gen/pos_iface/v1/posifacev1connect"
	"github.com/apotech/backend/gen/prescription_iface/v1/prescriptionifacev1connect"
	"github.com/apotech/backend/gen/purchasing_iface/v1/purchasingifacev1connect"
	"github.com/apotech/backend/gen/tax_iface/v1/taxifacev1connect"
	"github.com/apotech/backend/gen/user_iface/v1/userifacev1connect"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/config"
	"github.com/apotech/backend/internal/db"
	"github.com/apotech/backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.MustLoad()
	gormDB := db.MustOpen(cfg)

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
	salesAnalyticsSvc := service.NewSalesAnalytics(gormDB)
	inventoryAnalyticsSvc := service.NewInventoryAnalytics(gormDB)
	marginAnalyticsSvc := service.NewMarginAnalytics(gormDB)
	purchaseOrdersSvc := service.NewPurchaseOrders(gormDB)
	purchaseReceiptsSvc := service.NewPurchaseReceipts(gormDB)
	purchasePaymentsSvc := service.NewPurchasePayments(gormDB)
	prescriptionsSvc := service.NewPrescriptions(gormDB)
	taxInvoicesSvc := service.NewTaxInvoices(gormDB)
	bpjsClaimsSvc := service.NewBpjsClaims(gormDB)
	branchesSvc := service.NewBranches(gormDB)

	if err := userSvc.EnsureBootstrapOwner(context.Background(), cfg.Bootstrap); err != nil {
		log.Fatalf("bootstrap: %v", err) // intentionally fatal — server can't start
	}

	mux := http.NewServeMux()
	mux.Handle(healthifacev1connect.NewHealthServiceHandler(healthSvc, interceptors))
	mux.Handle(userifacev1connect.NewAuthServiceHandler(authSvc, interceptors))
	mux.Handle(userifacev1connect.NewUserServiceHandler(userSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewSupplierServiceHandler(supplierSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewMedicineServiceHandler(medicineSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewBatchServiceHandler(batchSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewStockMovementServiceHandler(stockSvc, interceptors))
	mux.Handle(customerifacev1connect.NewCustomerServiceHandler(customerSvc, interceptors))
	mux.Handle(posifacev1connect.NewSaleServiceHandler(saleSvc, interceptors))
	mux.Handle(analyticsifacev1connect.NewSalesAnalyticsServiceHandler(salesAnalyticsSvc, interceptors))
	mux.Handle(analyticsifacev1connect.NewInventoryAnalyticsServiceHandler(inventoryAnalyticsSvc, interceptors))
	mux.Handle(analyticsifacev1connect.NewMarginAnalyticsServiceHandler(marginAnalyticsSvc, interceptors))
	mux.Handle(purchasingifacev1connect.NewPurchaseOrderServiceHandler(purchaseOrdersSvc, interceptors))
	mux.Handle(purchasingifacev1connect.NewPurchaseReceiptServiceHandler(purchaseReceiptsSvc, interceptors))
	mux.Handle(purchasingifacev1connect.NewPurchasePaymentServiceHandler(purchasePaymentsSvc, interceptors))
	mux.Handle(prescriptionifacev1connect.NewPrescriptionServiceHandler(prescriptionsSvc, interceptors))
	mux.Handle(taxifacev1connect.NewTaxInvoiceServiceHandler(taxInvoicesSvc, interceptors))
	mux.Handle(bpjsifacev1connect.NewBpjsClaimServiceHandler(bpjsClaimsSvc, interceptors))
	mux.Handle(branchifacev1connect.NewBranchServiceHandler(branchesSvc, interceptors))

	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true) // h2c: HTTP/2 over plain TCP for gRPC/Connect streams

	addr := fmt.Sprintf("localhost:%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:      addr,
		Handler:   mux,
		Protocols: &protocols,
	}

	slog.Info("apotech listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
