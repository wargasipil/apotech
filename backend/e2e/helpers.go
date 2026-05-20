package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/apotech/backend/gen/inventory_iface/v1/inventoryifacev1connect"
	"github.com/apotech/backend/gen/stocktake_iface/v1/stocktakeifacev1connect"
	userifacev1 "github.com/apotech/backend/gen/user_iface/v1"
	"github.com/apotech/backend/gen/user_iface/v1/userifacev1connect"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/config"
	"github.com/apotech/backend/internal/db"
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
	Suppliers  inventoryifacev1connect.SupplierServiceClient
	Medicines  inventoryifacev1connect.MedicineServiceClient
	Batches    inventoryifacev1connect.BatchServiceClient
	Stocktakes stocktakeifacev1connect.StocktakeServiceClient
	Owner      TestUser
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
	gormDB, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
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
	supplierSvc := service.NewSuppliers(gormDB)
	medicineSvc := service.NewMedicines(gormDB)
	batchSvc := service.NewBatches(gormDB)
	stocktakeSvc := service.NewStocktakes(gormDB)

	if cfg.Bootstrap.OwnerEmail == "" {
		t.Fatalf("config.bootstrap.owner_email is empty; set it in config.yaml so tests have a known user")
	}
	if err := userSvc.EnsureBootstrapOwner(context.Background(), cfg.Bootstrap); err != nil {
		t.Fatalf("ensure bootstrap owner: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(userifacev1connect.NewAuthServiceHandler(authSvc, interceptors))
	mux.Handle(userifacev1connect.NewUserServiceHandler(userSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewSupplierServiceHandler(supplierSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewMedicineServiceHandler(medicineSvc, interceptors))
	mux.Handle(inventoryifacev1connect.NewBatchServiceHandler(batchSvc, interceptors))
	mux.Handle(stocktakeifacev1connect.NewStocktakeServiceHandler(stocktakeSvc, interceptors))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &Env{
		Server:    srv,
		DB:        gormDB,
		Auth:      userifacev1connect.NewAuthServiceClient(srv.Client(), srv.URL),
		Users:     userifacev1connect.NewUserServiceClient(srv.Client(), srv.URL),
		Suppliers:  inventoryifacev1connect.NewSupplierServiceClient(srv.Client(), srv.URL),
		Medicines:  inventoryifacev1connect.NewMedicineServiceClient(srv.Client(), srv.URL),
		Batches:    inventoryifacev1connect.NewBatchServiceClient(srv.Client(), srv.URL),
		Stocktakes: stocktakeifacev1connect.NewStocktakeServiceClient(srv.Client(), srv.URL),
		Owner: TestUser{
			Email:    cfg.Bootstrap.OwnerEmail,
			Password: cfg.Bootstrap.OwnerPassword,
		},
	}
}
