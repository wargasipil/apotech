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
)

func unitByName(units []*inventoryifacev1.MedicineUnit, name string) *inventoryifacev1.MedicineUnit {
	for _, u := range units {
		if u.Name == name {
			return u
		}
	}
	return nil
}

// TestMedicineUnits_DefineAndSellInUnits proves the Phase-1 UOM model: define
// base + larger units, sell in those units (price per unit), FEFO consumes the
// BASE-unit equivalent across batches, the sale line records the selling unit,
// and over-selling the base stock is rejected.
func TestMedicineUnits_DefineAndSellInUnits(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("UOM%d", uniq%100000))

	// Medicine: tablet (base, Rp700) + strip ×10 (Rp6500) + box ×100 (Rp60000).
	created, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("uom-%d", uniq), Name: fmt.Sprintf("UOM-Med-%d", uniq),
			Unit: "tablet", UnitPrice: 700,
			Units: []*inventoryifacev1.MedicineUnitInput{
				{Name: "strip", Factor: 10, SellPrice: 6500, Sellable: true, Purchasable: true},
				{Name: "box", Factor: 100, SellPrice: 60000, Sellable: true, Purchasable: true},
			},
		}))
	require.NoError(t, err)
	medID := created.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	got, err := env.Medicines.GetMedicine(ctx, whReq(env, t,
		&inventoryifacev1.GetMedicineRequest{Id: medID}, wh))
	require.NoError(t, err)
	units := got.Msg.Medicine.Units
	require.Len(t, units, 3, "base + strip + box")
	base := unitByName(units, "tablet")
	strip := unitByName(units, "strip")
	box := unitByName(units, "box")
	require.NotNil(t, base)
	require.NotNil(t, strip)
	require.NotNil(t, box)
	require.True(t, base.IsBase)
	require.Equal(t, int64(1), base.Factor)
	require.Equal(t, int64(10), strip.Factor)
	require.Equal(t, int64(100), box.Factor)

	// Two batches: 30 (earlier expiry) + 250 (later) = 280 base units.
	b1, err := env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "UOM-B1", ExpiryDate: "2027-01-31",
			CostPrice: 500, InitialQuantity: 30,
		}, wh))
	require.NoError(t, err)
	b2, err := env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "UOM-B2", ExpiryDate: "2028-01-31",
			CostPrice: 500, InitialQuantity: 250,
		}, wh))
	require.NoError(t, err)
	b1ID := b1.Msg.Batch.Id
	b2ID := b2.Msg.Batch.Id

	// Sell 2 strips = 20 base. FEFO takes from b1 (earlier expiry).
	sale1 := startSaleWith(env, t, ctx, wh)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: sale1, MedicineId: medID, MedicineUnitId: strip.Id, Qty: 2}, wh))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: sale1, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 13000,
		}, wh))
	require.NoError(t, err)

	require.Equal(t, int64(10), stockOf(env, t, ctx, b1ID, wh), "b1 30 - 20 = 10")
	require.Equal(t, int64(250), stockOf(env, t, ctx, b2ID, wh), "b2 untouched")

	gs1, err := env.Sales.GetSale(ctx, authReq(env, t, &posifacev1.GetSaleRequest{Id: sale1}))
	require.NoError(t, err)
	require.Len(t, gs1.Msg.Sale.Items, 1, "one line per selling unit (no per-batch split)")
	it := gs1.Msg.Sale.Items[0]
	require.Equal(t, "strip", it.UnitName)
	require.Equal(t, int32(2), it.Qty)
	require.Equal(t, int32(20), it.BaseQty)
	require.Equal(t, int64(6500), it.UnitPriceSnapshot)
	require.Equal(t, int64(13000), it.LineTotal)

	// Sell 1 box = 100 base. FEFO spans b1 (10) then b2 (90).
	sale2 := startSaleWith(env, t, ctx, wh)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: sale2, MedicineId: medID, MedicineUnitId: box.Id, Qty: 1}, wh))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: sale2, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 60000,
		}, wh))
	require.NoError(t, err)
	require.Equal(t, int64(0), stockOf(env, t, ctx, b1ID, wh), "b1 drained")
	require.Equal(t, int64(160), stockOf(env, t, ctx, b2ID, wh), "b2 250 - 90 = 160")

	gs2, err := env.Sales.GetSale(ctx, authReq(env, t, &posifacev1.GetSaleRequest{Id: sale2}))
	require.NoError(t, err)
	require.Len(t, gs2.Msg.Sale.Items, 1)
	require.Equal(t, "box", gs2.Msg.Sale.Items[0].UnitName)
	require.Equal(t, int32(100), gs2.Msg.Sale.Items[0].BaseQty)

	// Quantity aggregation counts BASE units: 2 strips (20) + 1 box (100) = 120,
	// NOT 3 (the selling-unit count). Scope to this medicine via the name query.
	now := time.Now()
	summ, err := env.Sales.GetSalesSummary(ctx, whReq(env, t,
		&posifacev1.GetSalesSummaryRequest{
			Status:   posifacev1.SaleStatus_SALE_STATUS_COMPLETED,
			Query:    fmt.Sprintf("UOM-Med-%d", uniq),
			FromUnix: now.AddDate(0, 0, -1).Unix(),
			ToUnix:   now.AddDate(0, 0, 1).Unix(),
		}, wh))
	require.NoError(t, err)
	require.Equal(t, int64(2), summ.Msg.SaleCount)
	require.Equal(t, int64(120), summ.Msg.ItemsSold, "items_sold counts base units (20 + 100), not 3")

	// Over-sell: 2 boxes = 200 base > 160 remaining → FailedPrecondition.
	sale3 := startSaleWith(env, t, ctx, wh)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: sale3, MedicineId: medID, MedicineUnitId: box.Id, Qty: 2}, wh))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: sale3, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 120000,
		}, wh))
	require.Error(t, err)
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeFailedPrecondition, cerr.Code())
}
