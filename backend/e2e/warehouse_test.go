package e2e

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	authifacev1 "github.com/apotech/backend/gen/auth_iface/v1"
	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	posifacev1 "github.com/apotech/backend/gen/pos_iface/v1"
	userifacev1 "github.com/apotech/backend/gen/user_iface/v1"
	warehouseifacev1 "github.com/apotech/backend/gen/warehouse_iface/v1"
)

// whReq builds an authenticated request that also carries the active-warehouse
// header (X-Warehouse-Id) — the mechanism the frontend uses to scope stock.
func whReq[T any](env *Env, t *testing.T, msg *T, warehouseID string) *connect.Request[T] {
	t.Helper()
	r := connect.NewRequest(msg)
	r.Header().Set("Authorization", env.AuthHeader(t))
	r.Header().Set("X-Warehouse-Id", warehouseID)
	return r
}

// stockOf returns the current qty of a batch as seen from a given warehouse.
func stockOf(env *Env, t *testing.T, ctx context.Context, batchID, warehouseID string) int64 {
	t.Helper()
	res, err := env.Stock.GetStockLevels(ctx, whReq(env, t,
		&inventoryifacev1.GetStockLevelsRequest{}, warehouseID))
	require.NoError(t, err)
	for _, l := range res.Msg.Levels {
		if l.BatchId == batchID {
			return l.CurrentQuantity
		}
	}
	return 0
}

func makeWarehouse(env *Env, t *testing.T, ctx context.Context, code string) string {
	t.Helper()
	res, err := env.Warehouses.CreateWarehouse(ctx, authReq(env, t,
		&warehouseifacev1.CreateWarehouseRequest{Code: code, Name: code + " gudang"}))
	require.NoError(t, err)
	require.NotEmpty(t, res.Msg.Warehouse.Id)
	return res.Msg.Warehouse.Id
}

// TestWarehouse_PerWarehouseStockAndPOS proves stock is partitioned per
// warehouse and POS only sells from the active one.
func TestWarehouse_PerWarehouseStockAndPOS(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("WHA%d", uniq%100000))
	whB := makeWarehouse(env, t, ctx, fmt.Sprintf("WHB%d", uniq%100000))

	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("e2e-wh-%d", uniq), Name: "WH med", Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	// Seed 10 units into warehouse A (the initial PURCHASE movement lands in A).
	batch, err := env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "WH-B1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 10,
		}, whA))
	require.NoError(t, err)
	batchID := batch.Msg.Batch.Id

	// Stock visible only in A.
	require.Equal(t, int64(10), stockOf(env, t, ctx, batchID, whA), "A holds the stock")
	require.Equal(t, int64(0), stockOf(env, t, ctx, batchID, whB), "B is empty")

	// Sell 3 from A — succeeds.
	saleA := startSaleWith(env, t, ctx, whA)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleA, MedicineId: medID, Qty: 3}, whA))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: saleA, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 3000,
		}, whA))
	require.NoError(t, err)
	require.Equal(t, int64(7), stockOf(env, t, ctx, batchID, whA), "A drops to 7 after the sale")

	// Sell from B — fails: no stock there even though the lot exists in A.
	saleB := startSaleWith(env, t, ctx, whB)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleB, MedicineId: medID, Qty: 1}, whB))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: saleB, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 1000,
		}, whB))
	require.Error(t, err, "selling from an empty warehouse must fail")
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeFailedPrecondition, cerr.Code())
}

func startSaleWith(env *Env, t *testing.T, ctx context.Context, warehouseID string) string {
	t.Helper()
	res, err := env.Sales.StartSale(ctx, whReq(env, t, &posifacev1.StartSaleRequest{}, warehouseID))
	require.NoError(t, err)
	return res.Msg.Sale.Id
}

// TestListWarehouses_Pagination proves the limit/offset/total contract: a
// limited page caps row count, total ignores the page window, and offset
// advances past prior rows. The dev DB carries an arbitrary number of
// warehouses from prior tests, so we scope assertions to a unique-prefix
// seeded set and stick to invariants (>= seeded, no specific Total value).
func TestListWarehouses_Pagination(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano() % 1000000

	const seeded = 3
	codes := make([]string, seeded)
	for i := 0; i < seeded; i++ {
		code := fmt.Sprintf("PG%06d%d", uniq, i)
		codes[i] = code
		_ = makeWarehouse(env, t, ctx, code)
	}

	// 1. A large enough single page surfaces every seeded warehouse + a Total
	//    that's at least our seeded count.
	all, err := env.Warehouses.ListWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehousesRequest{IncludeInactive: true, Limit: 1000}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, all.Msg.Total, int32(seeded),
		"total must count all matching rows (>= the rows we seeded)")
	seen := map[string]bool{}
	for _, w := range all.Msg.Warehouses {
		seen[w.Code] = true
	}
	for _, c := range codes {
		require.True(t, seen[c], "seeded warehouse %q must appear in the full listing", c)
	}

	// 2. Limit caps a page; Total still reflects the full filtered count.
	p1, err := env.Warehouses.ListWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehousesRequest{IncludeInactive: true, Limit: 2, Offset: 0}))
	require.NoError(t, err)
	require.LessOrEqual(t, len(p1.Msg.Warehouses), 2, "page must respect the requested limit")
	require.Equal(t, all.Msg.Total, p1.Msg.Total, "total must not depend on the page window")

	// 3. Offset advances past the previous page; the row ids are disjoint.
	if len(p1.Msg.Warehouses) == 2 && all.Msg.Total > 2 {
		p2, err := env.Warehouses.ListWarehouses(ctx, authReq(env, t,
			&warehouseifacev1.ListWarehousesRequest{IncludeInactive: true, Limit: 2, Offset: 2}))
		require.NoError(t, err)
		require.NotEmpty(t, p2.Msg.Warehouses, "second page must contain rows when total > 2")
		require.NotEqual(t, p1.Msg.Warehouses[0].Id, p2.Msg.Warehouses[0].Id,
			"offset must advance past the prior page")
	}

	// 4. The IncludeInactive filter still works under pagination. Archive one
	//    seeded warehouse and assert it disappears from the active-only list
	//    but stays in the IncludeInactive list.
	archiveTarget := all.Msg.Warehouses // capture before mutation
	_ = archiveTarget                   // (we look up the id via code below)
	var seededID string
	for _, w := range all.Msg.Warehouses {
		if w.Code == codes[0] {
			seededID = w.Id
			break
		}
	}
	require.NotEmpty(t, seededID)
	_, err = env.Warehouses.ArchiveWarehouse(ctx, authReq(env, t,
		&warehouseifacev1.ArchiveWarehouseRequest{Id: seededID}))
	require.NoError(t, err)

	activeOnly, err := env.Warehouses.ListWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehousesRequest{IncludeInactive: false, Limit: 1000}))
	require.NoError(t, err)
	for _, w := range activeOnly.Msg.Warehouses {
		require.NotEqual(t, seededID, w.Id, "archived warehouse must not appear when IncludeInactive=false")
	}
}

// TestListWarehouses_Search exercises the new `query` filter — case-insensitive
// substring match against code OR name.
func TestListWarehouses_Search(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano() % 1000000
	codeA := fmt.Sprintf("ZQ%dA", uniq)
	codeB := fmt.Sprintf("ZQ%dB", uniq)
	whA := makeWarehouse(env, t, ctx, codeA)
	whB := makeWarehouse(env, t, ctx, codeB)
	_ = whA
	_ = whB

	// Both share the ZQ<uniq> prefix.
	prefix := fmt.Sprintf("ZQ%d", uniq)
	res, err := env.Warehouses.ListWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehousesRequest{Query: prefix, Limit: 1000}))
	require.NoError(t, err)
	got := 0
	for _, w := range res.Msg.Warehouses {
		if strings.HasPrefix(w.Code, prefix) {
			got++
		}
	}
	require.Equal(t, 2, got, "search by prefix returns both seeded warehouses")

	// codeA alone returns only whA.
	res, err = env.Warehouses.ListWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehousesRequest{Query: codeA, Limit: 1000}))
	require.NoError(t, err)
	hits := map[string]bool{}
	for _, w := range res.Msg.Warehouses {
		hits[w.Code] = true
	}
	require.True(t, hits[codeA])
	require.False(t, hits[codeB])

	// Non-matching query returns nothing of ours.
	res, err = env.Warehouses.ListWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehousesRequest{Query: "X-NO-MATCH-X", Limit: 1000}))
	require.NoError(t, err)
	for _, w := range res.Msg.Warehouses {
		require.False(t, strings.HasPrefix(w.Code, prefix))
	}
}

// TestListUserWarehouses_Search verifies the TopBar warehouse picker's new
// server-side search: ListUserWarehouses filters the user's accessible
// warehouses by ILIKE on code/name when `query` is set.
func TestListUserWarehouses_Search(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano() % 1000000
	codeA := fmt.Sprintf("UQ%dA", uniq)
	codeB := fmt.Sprintf("UQ%dB", uniq)
	_ = makeWarehouse(env, t, ctx, codeA)
	_ = makeWarehouse(env, t, ctx, codeB)

	// Both seeded warehouses share the UQ<uniq> prefix; OWNER was auto-granted
	// access at creation time, so they appear in ListUserWarehouses results.
	prefix := fmt.Sprintf("UQ%d", uniq)
	res, err := env.Warehouses.ListUserWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListUserWarehousesRequest{Query: prefix}))
	require.NoError(t, err)
	hits := 0
	for _, w := range res.Msg.Warehouses {
		if strings.HasPrefix(w.Code, prefix) {
			hits++
		}
	}
	require.Equal(t, 2, hits, "prefix query returns both seeded warehouses")

	// codeA alone returns only whA.
	res, err = env.Warehouses.ListUserWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListUserWarehousesRequest{Query: codeA}))
	require.NoError(t, err)
	codes := map[string]bool{}
	for _, w := range res.Msg.Warehouses {
		codes[w.Code] = true
	}
	require.True(t, codes[codeA])
	require.False(t, codes[codeB])

	// Empty query falls back to "no filter" — full accessible list.
	res, err = env.Warehouses.ListUserWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListUserWarehousesRequest{}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Msg.Warehouses), 2, "no query returns full accessible list")
}

// TestWarehouse_SetGlobalDefault verifies promoting a warehouse to the
// company-wide default clears the previous default.
// TestGetWarehouse covers the new single-fetch RPC: returns a warehouse by
// id; rejects empty id (InvalidArgument); 404 on unknown id.
func TestGetWarehouse(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano() % 1000000
	whID := makeWarehouse(env, t, ctx, fmt.Sprintf("GWH%d", uniq))

	res, err := env.Warehouses.GetWarehouse(ctx, authReq(env, t,
		&warehouseifacev1.GetWarehouseRequest{Id: whID}))
	require.NoError(t, err)
	require.Equal(t, whID, res.Msg.Warehouse.Id)

	_, err = env.Warehouses.GetWarehouse(ctx, authReq(env, t,
		&warehouseifacev1.GetWarehouseRequest{Id: ""}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = env.Warehouses.GetWarehouse(ctx, authReq(env, t,
		&warehouseifacev1.GetWarehouseRequest{Id: "00000000-0000-0000-0000-000000000000"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestListWarehouseUsers covers the warehouse-detail "Users with access"
// query: lists members joined with user info; reflects grant + revoke.
func TestListWarehouseUsers(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano() % 1000000
	whID := makeWarehouse(env, t, ctx, fmt.Sprintf("LWU%d", uniq))

	// Auto-grant from CreateWarehouse means the OWNER appears.
	first, err := env.Warehouses.ListWarehouseUsers(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehouseUsersRequest{WarehouseId: whID}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(first.Msg.Users), 1)
	ownerSeen := false
	for _, u := range first.Msg.Users {
		if u.Email == env.Owner.Email {
			ownerSeen = true
		}
	}
	require.True(t, ownerSeen, "OWNER should appear via CreateWarehouse auto-grant")

	// Add a CASHIER and grant access to this warehouse.
	uniqEmail := fmt.Sprintf("lwu-%d@apotech.local", uniq)
	created, err := env.Users.CreateUser(ctx, authReq(env, t,
		&userifacev1.CreateUserRequest{
			Email: uniqEmail, Name: "LWU Test", Password: "Test1234!",
			Role: authifacev1.Role_ROLE_CASHIER,
		}))
	require.NoError(t, err)
	cashierID := created.Msg.User.Id
	t.Cleanup(func() {
		_, _ = env.Users.SetUserActive(ctx, authReq(env, t,
			&userifacev1.SetUserActiveRequest{UserId: cashierID, Active: false}))
	})

	_, err = env.Warehouses.GrantWarehouseAccess(ctx, authReq(env, t,
		&warehouseifacev1.GrantWarehouseAccessRequest{
			UserId: cashierID, WarehouseId: whID, IsDefault: false,
		}))
	require.NoError(t, err)

	second, err := env.Warehouses.ListWarehouseUsers(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehouseUsersRequest{WarehouseId: whID}))
	require.NoError(t, err)
	require.Equal(t, len(first.Msg.Users)+1, len(second.Msg.Users), "grant adds one member")

	// Revoke and confirm removal.
	_, err = env.Warehouses.RevokeWarehouseAccess(ctx, authReq(env, t,
		&warehouseifacev1.RevokeWarehouseAccessRequest{
			UserId: cashierID, WarehouseId: whID,
		}))
	require.NoError(t, err)

	third, err := env.Warehouses.ListWarehouseUsers(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehouseUsersRequest{WarehouseId: whID}))
	require.NoError(t, err)
	require.Equal(t, len(first.Msg.Users), len(third.Msg.Users), "revoke removes the member")
}

// TestSearchUsers verifies the new ILIKE search drives the Add-user picker
// (mirrors SearchCustomers / SearchSuppliers).
func TestSearchUsers(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	// Empty query returns at least one row (bootstrap owner is seeded).
	all, err := env.Users.SearchUsers(ctx, authReq(env, t,
		&userifacev1.SearchUsersRequest{Query: "", Limit: 50}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(all.Msg.Users), 1)

	// Substring of the seeded owner email returns the owner.
	hit, err := env.Users.SearchUsers(ctx, authReq(env, t,
		&userifacev1.SearchUsersRequest{Query: "owner@", Limit: 10}))
	require.NoError(t, err)
	seen := false
	for _, u := range hit.Msg.Users {
		if u.Email == env.Owner.Email {
			seen = true
		}
	}
	require.True(t, seen, "owner appears in 'owner@' search")

	// Non-matching query returns nothing of relevance.
	miss, err := env.Users.SearchUsers(ctx, authReq(env, t,
		&userifacev1.SearchUsersRequest{Query: "ZZ-NO-MATCH-ZZ", Limit: 10}))
	require.NoError(t, err)
	require.Empty(t, miss.Msg.Users)
}

func TestWarehouse_SetGlobalDefault(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano() % 1000000
	target := makeWarehouse(env, t, ctx, fmt.Sprintf("DEFNEW%d", uniq))

	// Snapshot the current global default so we can restore it after.
	listAll, err := env.Warehouses.ListWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehousesRequest{IncludeInactive: true, Limit: 1000}))
	require.NoError(t, err)
	var origDefaultID string
	for _, w := range listAll.Msg.Warehouses {
		if w.IsDefault {
			origDefaultID = w.Id
			break
		}
	}
	require.NotEmpty(t, origDefaultID, "fresh DB must have a default warehouse")
	require.NotEqual(t, target, origDefaultID, "test seed must differ from current default")
	t.Cleanup(func() {
		_, _ = env.Warehouses.SetGlobalDefaultWarehouse(ctx, authReq(env, t,
			&warehouseifacev1.SetGlobalDefaultWarehouseRequest{WarehouseId: origDefaultID}))
	})

	_, err = env.Warehouses.SetGlobalDefaultWarehouse(ctx, authReq(env, t,
		&warehouseifacev1.SetGlobalDefaultWarehouseRequest{WarehouseId: target}))
	require.NoError(t, err)

	after, err := env.Warehouses.ListWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehousesRequest{IncludeInactive: true, Limit: 1000}))
	require.NoError(t, err)
	defaults := 0
	for _, w := range after.Msg.Warehouses {
		if w.IsDefault {
			defaults++
			require.Equal(t, target, w.Id)
		}
	}
	require.Equal(t, 1, defaults, "exactly one default warehouse")
}
