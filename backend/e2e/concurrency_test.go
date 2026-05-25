package e2e

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	posifacev1 "github.com/apotech/backend/gen/pos_iface/v1"
)

// TestConcurrentCompleteSale_NoOversell proves the per-lot FOR UPDATE lock
// serializes concurrent sales of the same batch: two carts each buying the last
// unit, completed in parallel, must not both succeed and must not drive stock
// negative. Before the lock this oversold to -1 with both sales succeeding.
func TestConcurrentCompleteSale_NoOversell(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("CC%d", uniq%100000))

	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("e2e-cc-%d", uniq), Name: "Concurrency med", Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	// Exactly ONE unit in stock.
	batch, err := env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "CC-1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 1,
		}, whA))
	require.NoError(t, err)
	batchID := batch.Msg.Batch.Id

	// Two independent drafts, each wanting that one unit.
	const n = 2
	reqs := make([]*connect.Request[posifacev1.CompleteSaleRequest], n)
	for i := 0; i < n; i++ {
		saleID := startSaleWith(env, t, ctx, whA)
		_, err = env.Sales.AddItem(ctx, whReq(env, t,
			&posifacev1.AddItemRequest{SaleId: saleID, MedicineId: medID, Qty: 1}, whA))
		require.NoError(t, err)
		// Build the request (auth + warehouse headers) on the test goroutine so no
		// t.Fatalf fires from a worker goroutine.
		reqs[i] = whReq(env, t, &posifacev1.CompleteSaleRequest{
			SaleId:        saleID,
			PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
			PaidAmount:    100000,
		}, whA)
	}

	// Fire both completes in parallel.
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = env.Sales.CompleteSale(ctx, reqs[i])
		}(i)
	}
	wg.Wait()

	okCount, failCount := 0, 0
	for _, e := range errs {
		if e == nil {
			okCount++
			continue
		}
		var cerr *connect.Error
		require.True(t, errors.As(e, &cerr), "unexpected error type: %v", e)
		require.Equal(t, connect.CodeFailedPrecondition, cerr.Code(), "loser should be insufficient stock")
		failCount++
	}
	require.Equal(t, 1, okCount, "exactly one concurrent sale may succeed")
	require.Equal(t, 1, failCount, "the other must fail with insufficient stock")
	require.Equal(t, int64(0), stockOf(env, t, ctx, batchID, whA), "stock must never go negative")
}

// TestConcurrentUpdateMedicinePrice_NoSpuriousFailure proves the medicine-row
// lock serializes concurrent price edits: both succeed (no duplicate-key on the
// medicine_prices_open_idx) and exactly one open price row remains.
func TestConcurrentUpdateMedicinePrice_NoSpuriousFailure(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("e2e-ccp-%d", uniq), Name: "Price race med", Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	prices := []int64{2000, 3000}
	reqs := make([]*connect.Request[inventoryifacev1.UpdateMedicineRequest], len(prices))
	for i, p := range prices {
		reqs[i] = authReq(env, t, &inventoryifacev1.UpdateMedicineRequest{
			Id: medID, Name: "Price race med", Unit: "tab", UnitPrice: p,
		})
	}

	errs := make([]error, len(reqs))
	var wg sync.WaitGroup
	for i := range reqs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = env.Medicines.UpdateMedicine(ctx, reqs[i])
		}(i)
	}
	wg.Wait()

	for _, e := range errs {
		require.NoError(t, e, "concurrent price edits must serialize, not fail spuriously")
	}
	// Exactly one open (current) price row — the versioning stayed consistent.
	var openRows int64
	require.NoError(t, env.DB.Table("medicine_prices").
		Where("medicine_id = ? AND effective_to IS NULL", medID).
		Count(&openRows).Error)
	require.Equal(t, int64(1), openRows, "exactly one open price row must remain")
}
