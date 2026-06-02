package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	warehouseifacev1 "github.com/apotech/backend/gen/warehouse_iface/v1"
)

func flHasBatch(rows []*inventoryifacev1.Batch, id string) bool {
	for _, b := range rows {
		if b.Id == id {
			return true
		}
	}
	return false
}

func flHasMovement(rows []*inventoryifacev1.StockMovement, batchID string) bool {
	for _, mv := range rows {
		if mv.BatchId == batchID {
			return true
		}
	}
	return false
}

func flHasTransfer(rows []*warehouseifacev1.StockTransfer, id string) bool {
	for _, tr := range rows {
		if tr.Id == id {
			return true
		}
	}
	return false
}

// TestListFilters_SearchAndDateRange covers the new free-text search + date-range
// filters added to ListBatches, ListMovements, and ListTransfers.
func TestListFilters_SearchAndDateRange(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("FLA%d", uniq%100000))
	whB := makeWarehouse(env, t, ctx, fmt.Sprintf("FLB%d", uniq%100000))

	medName := fmt.Sprintf("FilterMed-%d", uniq)
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("fl-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	batchNo := fmt.Sprintf("FBATCH-%d", uniq)
	batch, err := env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: batchNo, ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 50,
		}, whA))
	require.NoError(t, err)
	batchID := batch.Msg.Batch.Id

	from := time.Now().AddDate(0, 0, -1).Unix()
	to := time.Now().AddDate(0, 0, 1).Unix()
	past := time.Now().AddDate(0, 0, -10).Unix()

	// --- ListBatches: search by medicine name + batch number; received-date range.
	byName, err := env.Batches.ListBatches(ctx, whReq(env, t,
		&inventoryifacev1.ListBatchesRequest{Query: medName}, whA))
	require.NoError(t, err)
	require.True(t, flHasBatch(byName.Msg.Batches, batchID), "search by medicine name finds the batch")

	byBatchNo, err := env.Batches.ListBatches(ctx, whReq(env, t,
		&inventoryifacev1.ListBatchesRequest{Query: batchNo}, whA))
	require.NoError(t, err)
	require.True(t, flHasBatch(byBatchNo.Msg.Batches, batchID), "search by batch number finds the batch")

	noMatch, err := env.Batches.ListBatches(ctx, whReq(env, t,
		&inventoryifacev1.ListBatchesRequest{Query: fmt.Sprintf("zz-no-match-%d", uniq)}, whA))
	require.NoError(t, err)
	require.False(t, flHasBatch(noMatch.Msg.Batches, batchID), "non-matching query excludes it")

	recvIn, err := env.Batches.ListBatches(ctx, whReq(env, t,
		&inventoryifacev1.ListBatchesRequest{Query: batchNo, DateField: "received", FromUnix: from, ToUnix: to}, whA))
	require.NoError(t, err)
	require.True(t, flHasBatch(recvIn.Msg.Batches, batchID), "received-date range includes today's batch")

	recvOut, err := env.Batches.ListBatches(ctx, whReq(env, t,
		&inventoryifacev1.ListBatchesRequest{Query: batchNo, DateField: "received", FromUnix: past, ToUnix: from}, whA))
	require.NoError(t, err)
	require.False(t, flHasBatch(recvOut.Msg.Batches, batchID), "past received-date window excludes it")

	// --- ListMovements: search by batch number / medicine name; created_at range.
	// Movements are scoped to the active warehouse, so query whA (where the batch
	// — and thus its PURCHASE movement — was seeded).
	mvByBatch, err := env.Stock.ListMovements(ctx, whReq(env, t,
		&inventoryifacev1.ListMovementsRequest{Query: batchNo}, whA))
	require.NoError(t, err)
	require.True(t, flHasMovement(mvByBatch.Msg.Movements, batchID), "movement search by batch number")

	mvByMed, err := env.Stock.ListMovements(ctx, whReq(env, t,
		&inventoryifacev1.ListMovementsRequest{Query: medName, FromUnix: from, ToUnix: to}, whA))
	require.NoError(t, err)
	require.True(t, flHasMovement(mvByMed.Msg.Movements, batchID), "movement search by medicine name + date range")

	mvOut, err := env.Stock.ListMovements(ctx, whReq(env, t,
		&inventoryifacev1.ListMovementsRequest{Query: batchNo, FromUnix: past, ToUnix: from}, whA))
	require.NoError(t, err)
	require.False(t, flHasMovement(mvOut.Msg.Movements, batchID), "past date window excludes the movement")

	// --- ListTransfers: search by transfer no / note; created_at range.
	note := fmt.Sprintf("FNOTE-%d", uniq)
	tr, err := env.Transfers.CreateTransfer(ctx, whReq(env, t,
		&warehouseifacev1.CreateTransferRequest{
			FromWarehouseId: whA, ToWarehouseId: whB, Note: note,
			Lines: []*warehouseifacev1.CreateTransferLineInput{{BatchId: batchID, Qty: 10}},
		}, whA))
	require.NoError(t, err)
	trID := tr.Msg.Transfer.Id
	trNo := tr.Msg.Transfer.TransferNo

	// ListTransfers is scoped to the active warehouse (from OR to). The transfer
	// is whA→whB, so query from whA's context.
	byNote, err := env.Transfers.ListTransfers(ctx, whReq(env, t,
		&warehouseifacev1.ListTransfersRequest{Query: note}, whA))
	require.NoError(t, err)
	require.True(t, flHasTransfer(byNote.Msg.Transfers, trID), "transfer search by note")

	byNo, err := env.Transfers.ListTransfers(ctx, whReq(env, t,
		&warehouseifacev1.ListTransfersRequest{Query: trNo, FromUnix: from, ToUnix: to}, whA))
	require.NoError(t, err)
	require.True(t, flHasTransfer(byNo.Msg.Transfers, trID), "transfer search by transfer_no + date range")

	trOut, err := env.Transfers.ListTransfers(ctx, whReq(env, t,
		&warehouseifacev1.ListTransfersRequest{Query: note, FromUnix: past, ToUnix: from}, whA))
	require.NoError(t, err)
	require.False(t, flHasTransfer(trOut.Msg.Transfers, trID), "past date window excludes the transfer")

	// Scoping hides transfers that don't touch the active warehouse: a third
	// warehouse's context must NOT see the whA→whB transfer.
	whC := makeWarehouse(env, t, ctx, fmt.Sprintf("FLC%d", uniq%100000))
	notHere, err := env.Transfers.ListTransfers(ctx, whReq(env, t,
		&warehouseifacev1.ListTransfersRequest{Query: note}, whC))
	require.NoError(t, err)
	require.False(t, flHasTransfer(notHere.Msg.Transfers, trID), "transfer absent from an unrelated warehouse's list")
}

// TestListBatches_SupplierFilter pins the new `supplier_id` filter on
// ListBatches: rows are constrained to lots from the specified supplier.
func TestListBatches_SupplierFilter(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	supA, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{
			Name: "Filter A", Code: fmt.Sprintf("FSA%d", uniq%100000),
		}))
	require.NoError(t, err)
	supB, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{
			Name: "Filter B", Code: fmt.Sprintf("FSB%d", uniq%100000),
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: supA.Msg.Supplier.Id}))
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: supB.Msg.Supplier.Id}))
	})

	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("fls-%d", uniq), Name: fmt.Sprintf("FLS-Med-%d", uniq),
			Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	batchA, err := env.Batches.CreateBatch(ctx, authReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, SupplierId: supA.Msg.Supplier.Id,
			BatchNumber: fmt.Sprintf("FLS-A-%d", uniq), ExpiryDate: "2099-12-31",
			CostPrice: 100, InitialQuantity: 5,
		}))
	require.NoError(t, err)
	batchB, err := env.Batches.CreateBatch(ctx, authReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, SupplierId: supB.Msg.Supplier.Id,
			BatchNumber: fmt.Sprintf("FLS-B-%d", uniq), ExpiryDate: "2099-12-31",
			CostPrice: 100, InitialQuantity: 5,
		}))
	require.NoError(t, err)

	// Filter on supA → only batchA.
	resA, err := env.Batches.ListBatches(ctx, authReq(env, t,
		&inventoryifacev1.ListBatchesRequest{
			MedicineId: medID, SupplierId: supA.Msg.Supplier.Id, Limit: 50,
		}))
	require.NoError(t, err)
	require.True(t, flHasBatch(resA.Msg.Batches, batchA.Msg.Batch.Id))
	require.False(t, flHasBatch(resA.Msg.Batches, batchB.Msg.Batch.Id))

	// Filter on supB → only batchB.
	resB, err := env.Batches.ListBatches(ctx, authReq(env, t,
		&inventoryifacev1.ListBatchesRequest{
			MedicineId: medID, SupplierId: supB.Msg.Supplier.Id, Limit: 50,
		}))
	require.NoError(t, err)
	require.True(t, flHasBatch(resB.Msg.Batches, batchB.Msg.Batch.Id))
	require.False(t, flHasBatch(resB.Msg.Batches, batchA.Msg.Batch.Id))

	// No supplier filter → both visible.
	resAll, err := env.Batches.ListBatches(ctx, authReq(env, t,
		&inventoryifacev1.ListBatchesRequest{MedicineId: medID, Limit: 50}))
	require.NoError(t, err)
	require.True(t, flHasBatch(resAll.Msg.Batches, batchA.Msg.Batch.Id))
	require.True(t, flHasBatch(resAll.Msg.Batches, batchB.Msg.Batch.Id))
}
