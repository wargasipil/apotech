package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
)

// TestInventory_ReadsScopedToActiveWarehouse proves the inventory reads that
// used to sum globally now honor the active warehouse (X-Warehouse-Id):
// ListMovements, GetBatch/SearchBatches qty, and GetMedicine
// total_stock/stock_valuation. One medicine, stock split across two warehouses.
func TestInventory_ReadsScopedToActiveWarehouse(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("IWA%d", uniq%100000))
	whB := makeWarehouse(env, t, ctx, fmt.Sprintf("IWB%d", uniq%100000))

	sku := fmt.Sprintf("e2e-invwh-%d", uniq)
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: sku, Name: "InvWH med", Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	// Batch 1: 10 units @ cost 500 → warehouse A. Batch 2: 5 @ 800 → warehouse B.
	b1, err := env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "IW-A1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 10,
		}, whA))
	require.NoError(t, err)
	b1ID := b1.Msg.Batch.Id

	b2, err := env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "IW-B1", ExpiryDate: "2099-12-31",
			CostPrice: 800, InitialQuantity: 5,
		}, whB))
	require.NoError(t, err)
	b2ID := b2.Msg.Batch.Id

	// GetBatch qty is per active warehouse.
	require.Equal(t, int64(10), getBatchQty(env, t, ctx, b1ID, whA), "b1 = 10 in A")
	require.Equal(t, int64(0), getBatchQty(env, t, ctx, b1ID, whB), "b1 = 0 in B")
	require.Equal(t, int64(5), getBatchQty(env, t, ctx, b2ID, whB), "b2 = 5 in B")
	require.Equal(t, int64(0), getBatchQty(env, t, ctx, b2ID, whA), "b2 = 0 in A")

	// SearchBatches reports per-active-warehouse qty (both batches match the sku).
	sb, err := env.Batches.SearchBatches(ctx, whReq(env, t,
		&inventoryifacev1.SearchBatchesRequest{Query: sku, Limit: 50}, whA))
	require.NoError(t, err)
	qtyBySearch := map[string]int64{}
	for _, b := range sb.Msg.Batches {
		qtyBySearch[b.Id] = b.CurrentQuantity
	}
	require.Equal(t, int64(10), qtyBySearch[b1ID], "search shows b1 = 10 in A")
	require.Equal(t, int64(0), qtyBySearch[b2ID], "search shows b2 = 0 in A")

	// ListMovements is scoped: A sees only b1's PURCHASE, B only b2's.
	lmA, err := env.Stock.ListMovements(ctx, whReq(env, t,
		&inventoryifacev1.ListMovementsRequest{MedicineId: medID, Limit: 100}, whA))
	require.NoError(t, err)
	require.Equal(t, int32(1), lmA.Msg.Total, "A sees only its own movement")
	for _, m := range lmA.Msg.Movements {
		require.Equal(t, b1ID, m.BatchId)
	}
	lmB, err := env.Stock.ListMovements(ctx, whReq(env, t,
		&inventoryifacev1.ListMovementsRequest{MedicineId: medID, Limit: 100}, whB))
	require.NoError(t, err)
	require.Equal(t, int32(1), lmB.Msg.Total, "B sees only its own movement")

	// GetMedicine total_stock + valuation are scoped to the active warehouse.
	gmA, err := env.Medicines.GetMedicine(ctx, whReq(env, t,
		&inventoryifacev1.GetMedicineRequest{Id: medID}, whA))
	require.NoError(t, err)
	require.Equal(t, int64(10), gmA.Msg.Medicine.TotalStock, "A total = 10")
	require.Equal(t, int64(5000), gmA.Msg.Medicine.StockValuation, "A valuation = 10*500")

	gmB, err := env.Medicines.GetMedicine(ctx, whReq(env, t,
		&inventoryifacev1.GetMedicineRequest{Id: medID}, whB))
	require.NoError(t, err)
	require.Equal(t, int64(5), gmB.Msg.Medicine.TotalStock, "B total = 5")
	require.Equal(t, int64(4000), gmB.Msg.Medicine.StockValuation, "B valuation = 5*800")
}

func getBatchQty(env *Env, t *testing.T, ctx context.Context, batchID, warehouseID string) int64 {
	t.Helper()
	res, err := env.Batches.GetBatch(ctx, whReq(env, t,
		&inventoryifacev1.GetBatchRequest{Id: batchID}, warehouseID))
	require.NoError(t, err)
	return res.Msg.Batch.CurrentQuantity
}
