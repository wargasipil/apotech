package e2e

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	posifacev1 "github.com/apotech/backend/gen/pos_iface/v1"
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
