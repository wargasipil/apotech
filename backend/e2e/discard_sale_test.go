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
	"github.com/apotech/backend/internal/service"
)

// seedDiscardMedicine creates a medicine with a stocked batch in the warehouse,
// returning the medicine id. Used by the discard + sweeper tests.
func seedDiscardMedicine(env *Env, t *testing.T, ctx context.Context, whID, tag string) string {
	t.Helper()
	uniq := time.Now().UnixNano()
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("e2e-%s-%d", tag, uniq), Name: tag + " med", Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})
	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: tag + "-1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 50,
		}, whID))
	require.NoError(t, err)
	return medID
}

// TestDiscardSale hard-deletes a DRAFT cart so it leaves no trace, and refuses
// to discard a COMPLETED sale.
func TestDiscardSale(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("DSA%d", time.Now().UnixNano()%100000))
	medID := seedDiscardMedicine(env, t, ctx, whA, "ds")

	// Draft with an item → discard → gone from GetSale + ListSales.
	draftID := startSaleWith(env, t, ctx, whA)
	_, err := env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: draftID, MedicineId: medID, Qty: 2}, whA))
	require.NoError(t, err)

	_, err = env.Sales.DiscardSale(ctx, whReq(env, t,
		&posifacev1.DiscardSaleRequest{SaleId: draftID}, whA))
	require.NoError(t, err)

	_, err = env.Sales.GetSale(ctx, whReq(env, t,
		&posifacev1.GetSaleRequest{Id: draftID}, whA))
	require.Error(t, err, "discarded sale is gone")
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeNotFound, cerr.Code())

	ls, err := env.Sales.ListSales(ctx, whReq(env, t,
		&posifacev1.ListSalesRequest{Limit: 1000}, whA))
	require.NoError(t, err)
	for _, s := range ls.Msg.Sales {
		require.NotEqual(t, draftID, s.Id, "discarded draft absent from list")
	}

	// Completed sale cannot be discarded.
	completedID := startSaleWith(env, t, ctx, whA)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: completedID, MedicineId: medID, Qty: 1}, whA))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: completedID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 1000,
		}, whA))
	require.NoError(t, err)

	_, err = env.Sales.DiscardSale(ctx, whReq(env, t,
		&posifacev1.DiscardSaleRequest{SaleId: completedID}, whA))
	require.Error(t, err, "completed sale cannot be discarded")
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeFailedPrecondition, cerr.Code())
}

// TestSweepStaleDrafts deletes drafts idle past the cutoff; recent drafts and
// completed sales survive.
func TestSweepStaleDrafts(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("SWA%d", time.Now().UnixNano()%100000))
	medID := seedDiscardMedicine(env, t, ctx, whA, "sw")

	// Old draft (backdated updated_at) — should be swept. Use raw SQL so GORM's
	// auto-update of updated_at doesn't reset it.
	oldDraft := startSaleWith(env, t, ctx, whA)
	require.NoError(t, env.DB.Exec(
		"UPDATE sales SET updated_at = ? WHERE id = ?",
		time.Now().Add(-48*time.Hour), oldDraft).Error)

	// Recent draft — should survive.
	recentDraft := startSaleWith(env, t, ctx, whA)
	t.Cleanup(func() {
		_, _ = env.Sales.DiscardSale(ctx, whReq(env, t,
			&posifacev1.DiscardSaleRequest{SaleId: recentDraft}, whA))
	})

	// Completed sale, even if old, survives (it's not a DRAFT).
	completedID := startSaleWith(env, t, ctx, whA)
	_, err := env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: completedID, MedicineId: medID, Qty: 1}, whA))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: completedID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 1000,
		}, whA))
	require.NoError(t, err)
	require.NoError(t, env.DB.Exec(
		"UPDATE sales SET updated_at = ? WHERE id = ?",
		time.Now().Add(-48*time.Hour), completedID).Error)

	deleted, err := service.SweepStaleDrafts(ctx, env.DB, 24*time.Hour)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))

	// Old draft gone.
	_, err = env.Sales.GetSale(ctx, whReq(env, t,
		&posifacev1.GetSaleRequest{Id: oldDraft}, whA))
	require.Error(t, err, "old draft swept")
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeNotFound, cerr.Code())

	// Recent draft survives.
	_, err = env.Sales.GetSale(ctx, whReq(env, t,
		&posifacev1.GetSaleRequest{Id: recentDraft}, whA))
	require.NoError(t, err, "recent draft survives")

	// Completed sale survives.
	_, err = env.Sales.GetSale(ctx, whReq(env, t,
		&posifacev1.GetSaleRequest{Id: completedID}, whA))
	require.NoError(t, err, "completed sale survives")
}
