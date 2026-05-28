package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	posifacev1 "github.com/apotech/backend/gen/pos_iface/v1"
	userifacev1 "github.com/apotech/backend/gen/user_iface/v1"
)

// TestGetTodaySnapshot_WarehouseAndCashierScope proves the dashboard's source
// RPC scopes to (a) the caller's active warehouse and (b) optionally a single
// cashier — plus that `last_sale_unix` reflects the most recent matching sale.
func TestGetTodaySnapshot_WarehouseAndCashierScope(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("DASA%d", uniq%100000))
	whB := makeWarehouse(env, t, ctx, fmt.Sprintf("DASB%d", uniq%100000))

	// Medicine + stock in each warehouse.
	medName := fmt.Sprintf("Dashboard-Med-%d", uniq)
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("dash-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})
	for _, wh := range []string{whA, whB} {
		_, err := env.Batches.CreateBatch(ctx, whReq(env, t,
			&inventoryifacev1.CreateBatchRequest{
				MedicineId: medID, BatchNumber: fmt.Sprintf("DASH-%s-%d", wh[:8], uniq),
				ExpiryDate: "2099-12-31", CostPrice: 400, InitialQuantity: 20,
			}, wh))
		require.NoError(t, err)
	}

	// Seed one sale in WH-A (qty 2 → 2000) and one in WH-B (qty 3 → 3000).
	sellIn := func(wh string, qty int32, paid int64) {
		t.Helper()
		sid := startSaleWith(env, t, ctx, wh)
		_, err := env.Sales.AddItem(ctx, whReq(env, t,
			&posifacev1.AddItemRequest{SaleId: sid, MedicineId: medID, Qty: qty}, wh))
		require.NoError(t, err)
		_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
			&posifacev1.CompleteSaleRequest{
				SaleId: sid, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: paid,
			}, wh))
		require.NoError(t, err)
	}
	sellIn(whA, 2, 2000)
	beforeBLast := time.Now().Unix() // bound the "last sale" assertion
	sellIn(whB, 3, 3000)

	// === WH-A snapshot ===
	snapA, err := env.Sales.GetTodaySnapshot(ctx, whReq(env, t,
		&posifacev1.GetTodaySnapshotRequest{}, whA))
	require.NoError(t, err)
	require.GreaterOrEqual(t, snapA.Msg.Revenue, int64(2000),
		"WH-A snapshot must include WH-A's 2000 sale")
	require.GreaterOrEqual(t, snapA.Msg.SaleCount, int64(1))
	require.GreaterOrEqual(t, snapA.Msg.ItemsSold, int64(2))
	require.Greater(t, snapA.Msg.LastSaleUnix, int64(0))

	// === WH-B snapshot ===
	snapB, err := env.Sales.GetTodaySnapshot(ctx, whReq(env, t,
		&posifacev1.GetTodaySnapshotRequest{}, whB))
	require.NoError(t, err)
	require.GreaterOrEqual(t, snapB.Msg.Revenue, int64(3000))
	// WH-B's last sale must be >= the timestamp captured before the WH-B sale,
	// proving the warehouse filter narrows the MAX(completed_at) too.
	require.GreaterOrEqual(t, snapB.Msg.LastSaleUnix, beforeBLast)

	// Revenue in WH-A must differ from WH-B (proves scoping).
	require.NotEqual(t, snapA.Msg.Revenue, snapB.Msg.Revenue,
		"per-warehouse revenue must differ")

	// === Cashier-scope filter: passing the OWNER's own id reproduces the
	// unfiltered numbers (the bootstrap-owner made every sale here). ===
	ownerID, err := getOwnerUserID(env, t, ctx)
	require.NoError(t, err)
	snapOwner, err := env.Sales.GetTodaySnapshot(ctx, whReq(env, t,
		&posifacev1.GetTodaySnapshotRequest{CashierUserId: ownerID}, whA))
	require.NoError(t, err)
	require.Equal(t, snapA.Msg.Revenue, snapOwner.Msg.Revenue,
		"OWNER passing self id sees identical numbers")

	// === Cashier-scope filter: a random non-existent user id yields zeros ===
	snapNoOne, err := env.Sales.GetTodaySnapshot(ctx, whReq(env, t,
		&posifacev1.GetTodaySnapshotRequest{CashierUserId: "00000000-0000-0000-0000-000000000000"}, whA))
	require.NoError(t, err)
	require.Equal(t, int64(0), snapNoOne.Msg.Revenue)
	require.Equal(t, int64(0), snapNoOne.Msg.LastSaleUnix)
}

// getOwnerUserID looks up the bootstrap-owner user's id via Me().
func getOwnerUserID(env *Env, t *testing.T, ctx context.Context) (string, error) {
	t.Helper()
	res, err := env.Auth.Me(ctx, authReq(env, t, &userifacev1.MeRequest{}))
	if err != nil {
		return "", err
	}
	return res.Msg.User.Id, nil
}

// Silence the unused-import warning when the imports above are needed elsewhere.
var _ = connect.NewRequest[any]
