package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"connectrpc.com/connect"

	"github.com/apotech/backend/gen/customer_iface/v1/customerifacev1connect"
	"github.com/apotech/backend/gen/health_iface/v1/healthifacev1connect"
	"github.com/apotech/backend/gen/inventory_iface/v1/inventoryifacev1connect"
	"github.com/apotech/backend/gen/pos_iface/v1/posifacev1connect"
	"github.com/apotech/backend/gen/user_iface/v1/userifacev1connect"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/config"
	"github.com/apotech/backend/internal/db"
	"github.com/apotech/backend/internal/service"
)

func main() {
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
	log.Printf("auth: %d procedures registered", len(policy))
	interceptors := connect.WithInterceptors(auth.NewInterceptor(issuer, policy))

	userSvc := service.NewUsers(gormDB)
	authSvc := service.NewAuth(gormDB, issuer, refreshIssuer)
	healthSvc := service.NewHealth(gormDB)
	supplierSvc := service.NewSuppliers(gormDB)
	medicineSvc := service.NewMedicines(gormDB)
	batchSvc := service.NewBatches(gormDB)
	stockSvc := service.NewStock(gormDB)
	customerSvc := service.NewCustomers(gormDB)
	saleSvc := service.NewSales(gormDB)

	if err := userSvc.EnsureBootstrapOwner(context.Background(), cfg.Bootstrap); err != nil {
		log.Fatalf("bootstrap: %v", err)
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

	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true) // h2c: HTTP/2 over plain TCP for gRPC/Connect streams

	addr := fmt.Sprintf("localhost:%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:      addr,
		Handler:   mux,
		Protocols: &protocols,
	}

	log.Printf("apotech: listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
