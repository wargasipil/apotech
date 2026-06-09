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
	"github.com/apotech/backend/internal/model"
)

// TestVoidSale_CompletedReversesStock pins the new behaviour: VoidSale now
// accepts COMPLETED sales and reverses each SALE stock_movements row by
// inserting an opposing ADJUSTMENT row linked to the same sale_item. The sale
// flips to VOIDED with a non-nil cancelled_at. NSFP is deliberately NOT
// released (DJP rules — see service comment).
func TestVoidSale_CompletedReversesStock(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("CNS%d", time.Now().UnixNano()%100000))
	medID := seedDiscardMedicine(env, t, ctx, whA, "cancel")

	// Complete a sale of 2 units (batch holds 50). Ready should drop to 48.
	saleID := startSaleWith(env, t, ctx, whA)
	_, err := env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleID, MedicineId: medID, Qty: 2}, whA))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId:        saleID,
			PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
			PaidAmount:    2000,
		}, whA))
	require.NoError(t, err)

	gotPre, err := env.Medicines.GetMedicine(ctx, whReq(env, t,
		&inventoryifacev1.GetMedicineRequest{Id: medID}, whA))
	require.NoError(t, err)
	require.Equal(t, int64(48), gotPre.Msg.Medicine.ReadyStock, "stock after sale = 50 - 2")

	// Cancel the COMPLETED sale.
	_, err = env.Sales.VoidSale(ctx, whReq(env, t,
		&posifacev1.VoidSaleRequest{SaleId: saleID}, whA))
	require.NoError(t, err)

	// Status flipped to VOIDED + cancelled_at populated.
	post, err := env.Sales.GetSale(ctx, whReq(env, t,
		&posifacev1.GetSaleRequest{Id: saleID}, whA))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_VOIDED, post.Msg.Sale.Status)
	require.Greater(t, post.Msg.Sale.CancelledAt, int64(0),
		"cancelled_at must be set after cancelling a completed sale")

	// Ready stock restored to 50.
	gotPost, err := env.Medicines.GetMedicine(ctx, whReq(env, t,
		&inventoryifacev1.GetMedicineRequest{Id: medID}, whA))
	require.NoError(t, err)
	require.Equal(t, int64(50), gotPost.Msg.Medicine.ReadyStock, "stock back to 50 after cancel")

	// Reverse ADJUSTMENT movements landed linked to the sale's items, with the
	// sale_no in the reason. The SALE movements stay; the reversals are
	// additional rows.
	var sale model.Sale
	require.NoError(t, env.DB.Where("id = ?", saleID).First(&sale).Error)
	var items []model.SaleItem
	require.NoError(t, env.DB.Where("sale_id = ?", saleID).Find(&items).Error)
	require.NotEmpty(t, items)

	itemIDs := make([]string, 0, len(items))
	for _, it := range items {
		itemIDs = append(itemIDs, it.ID)
	}
	var adjustments []model.StockMovement
	require.NoError(t, env.DB.
		Where("sale_item_id IN ? AND type = ?", itemIDs, "ADJUSTMENT").
		Find(&adjustments).Error)
	require.NotEmpty(t, adjustments, "expected at least one ADJUSTMENT reversal")
	var totalAdj int32
	for _, m := range adjustments {
		require.Greater(t, m.Qty, int32(0), "reversal qty is positive")
		require.Contains(t, m.Reason, "Sale cancelled", "reason marks the cancellation")
		if sale.SaleNo != nil {
			require.Contains(t, m.Reason, *sale.SaleNo, "reason embeds the sale_no")
		}
		totalAdj += m.Qty
	}
	require.Equal(t, int32(2), totalAdj, "total reversal qty == sold qty")

	// Re-cancelling a VOIDED sale is rejected.
	_, err = env.Sales.VoidSale(ctx, whReq(env, t,
		&posifacev1.VoidSaleRequest{SaleId: saleID}, whA))
	require.Error(t, err)
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeFailedPrecondition, cerr.Code())
}
