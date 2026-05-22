package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	posifacev1 "github.com/apotech/backend/gen/pos_iface/v1"
	purchasingifacev1 "github.com/apotech/backend/gen/purchasing_iface/v1"
)

// TestListMedicines_Pagination seeds more rows than a page holds and asserts
// limit/offset/total behave: page 1 and page 2 are disjoint and total counts
// every matching row regardless of the page window.
func TestListMedicines_Pagination(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()
	// A shared name token scopes the query to just these rows.
	token := fmt.Sprintf("PAGE-%d", uniq)

	const seeded = 5
	for i := 0; i < seeded; i++ {
		med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
			&inventoryifacev1.CreateMedicineRequest{
				Sku:       fmt.Sprintf("pg-%d-%d", uniq, i),
				Name:      fmt.Sprintf("%s item %02d", token, i),
				Unit:      "tab",
				UnitPrice: 1000,
			}))
		require.NoError(t, err)
		id := med.Msg.Medicine.Id
		t.Cleanup(func() {
			_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
				&inventoryifacev1.ArchiveMedicineRequest{Id: id}))
		})
	}

	// Page 1: limit 2, offset 0.
	p1, err := env.Medicines.ListMedicines(ctx, authReq(env, t,
		&inventoryifacev1.ListMedicinesRequest{Query: token, Limit: 2, Offset: 0}))
	require.NoError(t, err)
	require.Len(t, p1.Msg.Medicines, 2)
	require.Equal(t, int32(seeded), p1.Msg.Total, "total counts all matching rows")

	// Page 2: limit 2, offset 2 — disjoint from page 1.
	p2, err := env.Medicines.ListMedicines(ctx, authReq(env, t,
		&inventoryifacev1.ListMedicinesRequest{Query: token, Limit: 2, Offset: 2}))
	require.NoError(t, err)
	require.Len(t, p2.Msg.Medicines, 2)
	require.Equal(t, int32(seeded), p2.Msg.Total)
	require.NotEqual(t, p1.Msg.Medicines[0].Id, p2.Msg.Medicines[0].Id, "pages must differ")
	require.NotEqual(t, p1.Msg.Medicines[1].Id, p2.Msg.Medicines[0].Id)

	// Page 3: the remainder.
	p3, err := env.Medicines.ListMedicines(ctx, authReq(env, t,
		&inventoryifacev1.ListMedicinesRequest{Query: token, Limit: 2, Offset: 4}))
	require.NoError(t, err)
	require.Len(t, p3.Msg.Medicines, 1)
	require.Equal(t, int32(seeded), p3.Msg.Total)
}

// TestListMedicines_ReadyAndOnOrder proves the enriched stock columns: ready
// reflects on-hand in the active warehouse, on_order reflects open-PO quantity.
func TestListMedicines_ReadyAndOnOrder(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("STK%d", uniq%100000))

	medName := fmt.Sprintf("Stock-Med-%d", uniq)
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("stk-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	// Seed 8 on-hand into this warehouse.
	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "STK-B1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 8,
		}, wh))
	require.NoError(t, err)

	// Create + send a PO for 5 more (open → counts as on-order).
	sup, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{
			Name: "Stock sup", Code: fmt.Sprintf("STKSUP%d", uniq%100000),
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: sup.Msg.Supplier.Id}))
	})
	po, err := env.POs.CreatePurchaseOrder(ctx, authReq(env, t,
		&purchasingifacev1.CreatePurchaseOrderRequest{
			SupplierId: sup.Msg.Supplier.Id,
			Items: []*purchasingifacev1.PurchaseOrderItemInput{
				{MedicineId: medID, OrderedQty: 5, UnitCostPrice: 500},
			},
		}))
	require.NoError(t, err)
	_, err = env.POs.SendPurchaseOrder(ctx, authReq(env, t,
		&purchasingifacev1.SendPurchaseOrderRequest{Id: po.Msg.Order.Id}))
	require.NoError(t, err)

	// List from this warehouse: ready = 8, on_order = 5.
	res, err := env.Medicines.ListMedicines(ctx, whReq(env, t,
		&inventoryifacev1.ListMedicinesRequest{Query: medName}, wh))
	require.NoError(t, err)
	var got *inventoryifacev1.Medicine
	for _, m := range res.Msg.Medicines {
		if m.Id == medID {
			got = m
		}
	}
	require.NotNil(t, got, "seeded medicine should be in the list")
	require.Equal(t, int64(8), got.ReadyStock, "ready = on-hand in active warehouse")
	require.Equal(t, int64(5), got.OnOrderStock, "on_order = open PO outstanding qty")
}

// TestListSales_SearchAndDateRange proves a completed sale is findable by the
// medicine sold and by a created-date range.
func TestListSales_SearchAndDateRange(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("SAL%d", uniq%100000))

	medName := fmt.Sprintf("Sold-Med-%d", uniq)
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("sal-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "SAL-B1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 10,
		}, wh))
	require.NoError(t, err)

	saleID := startSaleWith(env, t, ctx, wh)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleID, MedicineId: medID, Qty: 2}, wh))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 2000,
		}, wh))
	require.NoError(t, err)

	// Search by the medicine name finds the sale and denormalizes the item name.
	res, err := env.Sales.ListSales(ctx, authReq(env, t,
		&posifacev1.ListSalesRequest{Query: medName}))
	require.NoError(t, err)
	got := findSale(res.Msg.Sales, saleID)
	require.NotNil(t, got, "search by medicine name should return the sale")
	require.GreaterOrEqual(t, res.Msg.Total, int32(1))
	foundItem := false
	for _, it := range got.Items {
		if it.MedicineName == medName {
			foundItem = true
		}
	}
	require.True(t, foundItem, "sale item should carry the denormalized medicine name")

	// Created-date range includes it.
	from := time.Now().AddDate(0, 0, -1).Unix()
	to := time.Now().AddDate(0, 0, 2).Unix()
	res, err = env.Sales.ListSales(ctx, authReq(env, t,
		&posifacev1.ListSalesRequest{Query: medName, FromUnix: from, ToUnix: to}))
	require.NoError(t, err)
	require.NotNil(t, findSale(res.Msg.Sales, saleID), "date range should include the sale")
}

func findSale(rows []*posifacev1.Sale, id string) *posifacev1.Sale {
	for _, r := range rows {
		if r.Id == id {
			return r
		}
	}
	return nil
}

// TestGetSalesSummary proves the order-history summary aggregates over ALL
// matching rows (server-side), honoring the same status/date/search filters as
// ListSales — not a client-side sum of a page.
func TestGetSalesSummary(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("SUM%d", uniq%100000))

	medName := fmt.Sprintf("Summary-Med-%d", uniq)
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("sum-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "SUM-B1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 10,
		}, wh))
	require.NoError(t, err)

	saleID := startSaleWith(env, t, ctx, wh)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleID, MedicineId: medID, Qty: 3}, wh))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 3000,
		}, wh))
	require.NoError(t, err)

	// Scope the summary to this medicine (query=medName) so the figures are exact:
	// 1 sale, 3 units sold, Rp 3000 revenue.
	sum, err := env.Sales.GetSalesSummary(ctx, authReq(env, t,
		&posifacev1.GetSalesSummaryRequest{
			Query:  medName,
			Status: posifacev1.SaleStatus_SALE_STATUS_COMPLETED,
		}))
	require.NoError(t, err)
	require.Equal(t, int64(1), sum.Msg.SaleCount, "exactly one matching sale")
	require.Equal(t, int64(3), sum.Msg.ItemsSold, "sum of qty")
	require.Equal(t, int64(3000), sum.Msg.Revenue, "sum of total")

	// A date window that ends before the sale was created → all zeros.
	past := time.Now().AddDate(0, 0, -10).Unix()
	pastEnd := time.Now().AddDate(0, 0, -5).Unix()
	empty, err := env.Sales.GetSalesSummary(ctx, authReq(env, t,
		&posifacev1.GetSalesSummaryRequest{
			Query:    medName,
			Status:   posifacev1.SaleStatus_SALE_STATUS_COMPLETED,
			FromUnix: past,
			ToUnix:   pastEnd,
		}))
	require.NoError(t, err)
	require.Equal(t, int64(0), empty.Msg.SaleCount)
	require.Equal(t, int64(0), empty.Msg.ItemsSold)
	require.Equal(t, int64(0), empty.Msg.Revenue)
}

// TestGetMedicine_EnrichAndMovementsByMedicine proves the detail page's two new
// backend bits: GetMedicine fills ready_stock (active warehouse), and
// ListMovements{medicine_id} returns that medicine's movements (and excludes
// another medicine's).
func TestGetMedicine_EnrichAndMovementsByMedicine(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("MED%d", uniq%100000))

	mk := func(suffix string) string {
		m, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
			&inventoryifacev1.CreateMedicineRequest{
				Sku:  fmt.Sprintf("md-%d-%s", uniq, suffix),
				Name: fmt.Sprintf("Detail-Med-%d-%s", uniq, suffix),
				Unit: "tab", UnitPrice: 1000,
			}))
		require.NoError(t, err)
		id := m.Msg.Medicine.Id
		t.Cleanup(func() {
			_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
				&inventoryifacev1.ArchiveMedicineRequest{Id: id}))
		})
		return id
	}
	medA := mk("a")
	medB := mk("b")

	// A supplier so the batch (and thus last_restock_supplier) has one.
	sup, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{
			Name: "Restock Sup", Code: fmt.Sprintf("RST%d", uniq%100000),
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: sup.Msg.Supplier.Id}))
	})

	// Seed 7 of medA into this warehouse (creates a PURCHASE movement of +7),
	// received on a known date from the supplier.
	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medA, BatchNumber: "MD-A1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 7,
			SupplierId: sup.Msg.Supplier.Id, ReceivedAt: "2026-05-20",
		}, wh))
	require.NoError(t, err)

	// GetMedicine from this warehouse → ready_stock + last-restock enriched.
	got, err := env.Medicines.GetMedicine(ctx, whReq(env, t,
		&inventoryifacev1.GetMedicineRequest{Id: medA}, wh))
	require.NoError(t, err)
	require.Equal(t, int64(7), got.Msg.Medicine.ReadyStock, "GetMedicine fills ready_stock")
	require.Equal(t, "2026-05-20", got.Msg.Medicine.LastRestockDate, "last restock date")
	require.Equal(t, "Restock Sup", got.Msg.Medicine.LastRestockSupplier, "last restock supplier")
	require.Equal(t, int64(7), got.Msg.Medicine.TotalStock, "total stock (all warehouses)")
	require.Equal(t, int64(3500), got.Msg.Medicine.StockValuation, "valuation = 7 × 500")

	// medB has no batch → no last-restock.
	gotB, err := env.Medicines.GetMedicine(ctx, whReq(env, t,
		&inventoryifacev1.GetMedicineRequest{Id: medB}, wh))
	require.NoError(t, err)
	require.Empty(t, gotB.Msg.Medicine.LastRestockDate)
	require.Empty(t, gotB.Msg.Medicine.LastRestockSupplier)

	// ListMovements{medicine_id: medA} → at least the PURCHASE movement; all rows
	// belong to medA's batch (medB has none).
	mv, err := env.Stock.ListMovements(ctx, authReq(env, t,
		&inventoryifacev1.ListMovementsRequest{MedicineId: medA}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(mv.Msg.Movements), 1)
	require.GreaterOrEqual(t, mv.Msg.Total, int32(1))

	mvB, err := env.Stock.ListMovements(ctx, authReq(env, t,
		&inventoryifacev1.ListMovementsRequest{MedicineId: medB}))
	require.NoError(t, err)
	require.Equal(t, int32(0), mvB.Msg.Total, "medB has no movements")
}
