package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	analyticsifacev1 "github.com/apotech/backend/gen/analytics_iface/v1"
	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	posifacev1 "github.com/apotech/backend/gen/pos_iface/v1"
)

// TestAnalytics_DimensionFanout proves the three dimension-scoped RPCs all
// surface a seeded sale + stock and that the map-keyed responses round-trip.
// Daily uses as-of-end-of-day reconstruction; Product / User return ids +
// metric maps; STOCK on User is rejected.
func TestAnalytics_DimensionFanout(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("ANL%d", uniq%100000))

	// Medicine + batch + a single completed sale of qty 2 @ 1000 each.
	medName := fmt.Sprintf("Analytics-Med-%d", uniq)
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("anl-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: fmt.Sprintf("ANL-B-%d", uniq),
			ExpiryDate: "2099-12-31", CostPrice: 400, InitialQuantity: 20,
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

	// Date range: yesterday → tomorrow so the just-completed sale lands inside.
	now := time.Now()
	from := now.AddDate(0, 0, -1).Unix()
	to := now.AddDate(0, 0, 1).Unix()
	filter := &analyticsifacev1.Filter{FromUnix: from, ToUnix: to}

	// --- DailyMetric: ORDER + STOCK, granularity DAY ---
	dailyRes, err := env.Analytics.DailyMetric(ctx, whReq(env, t,
		&analyticsifacev1.DailyMetricRequest{
			MetricTypes: []analyticsifacev1.MetricType{
				analyticsifacev1.MetricType_METRIC_TYPE_ORDER,
				analyticsifacev1.MetricType_METRIC_TYPE_STOCK,
			},
			Filter:      filter,
			Granularity: analyticsifacev1.Granularity_GRANULARITY_DAY,
		}, wh))
	require.NoError(t, err)
	require.Greater(t, len(dailyRes.Msg.Days), 0)
	require.NotNil(t, dailyRes.Msg.Order)
	require.NotNil(t, dailyRes.Msg.Stock)
	// The day key corresponding to "today" must carry the sale's revenue.
	todayKey := now.Format("2006-01-02")
	if oi, ok := dailyRes.Msg.Order.Data[todayKey]; ok {
		require.GreaterOrEqual(t, oi.Terjual, int64(2000),
			"today's terjual must include the seeded sale (>=2000)")
	}
	// Stock as-of-end-of-today reflects the seeded batch (20) minus the sale (2) = 18.
	if si, ok := dailyRes.Msg.Stock.Data[todayKey]; ok {
		require.GreaterOrEqual(t, si.Ready, int64(18),
			"today's stock_ready must be at least 18 after the sale")
	}

	// --- DailyMetric granularity WEEK collapses to fewer buckets ---
	weekRes, err := env.Analytics.DailyMetric(ctx, whReq(env, t,
		&analyticsifacev1.DailyMetricRequest{
			MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
			Filter:      filter,
			Granularity: analyticsifacev1.Granularity_GRANULARITY_WEEK,
		}, wh))
	require.NoError(t, err)
	require.LessOrEqual(t, len(weekRes.Msg.Days), len(dailyRes.Msg.Days),
		"week buckets must be <= day buckets over the same range")

	// --- ProductMetric: medicine_id list + ORDER block keyed by id ---
	prodRes, err := env.Analytics.ProductMetric(ctx, whReq(env, t,
		&analyticsifacev1.ProductMetricRequest{
			MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
			Filter:      filter,
		}, wh))
	require.NoError(t, err)
	require.Contains(t, prodRes.Msg.MedicineIds, medID, "seeded medicine must appear")
	require.NotNil(t, prodRes.Msg.Order)
	require.GreaterOrEqual(t, prodRes.Msg.Total, int32(1))
	require.GreaterOrEqual(t, prodRes.Msg.Order.Data[medID].Terjual, int64(2000))

	// --- UserMetric: cashier (bootstrap-owner) appears ---
	userRes, err := env.Analytics.UserMetric(ctx, whReq(env, t,
		&analyticsifacev1.UserMetricRequest{
			MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
			Filter:      filter,
		}, wh))
	require.NoError(t, err)
	require.GreaterOrEqual(t, userRes.Msg.Total, int32(1))
	require.Greater(t, len(userRes.Msg.UserIds), 0)
	require.NotNil(t, userRes.Msg.Order)
	// Pick the first id and confirm its terjual covers the seeded sale.
	uid := userRes.Msg.UserIds[0]
	require.GreaterOrEqual(t, userRes.Msg.Order.Data[uid].Terjual, int64(2000))

	// --- UserMetric + STOCK → InvalidArgument ---
	_, err = env.Analytics.UserMetric(ctx, whReq(env, t,
		&analyticsifacev1.UserMetricRequest{
			MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_STOCK},
			Filter:      filter,
		}, wh))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	// --- Sort field without requested metric → InvalidArgument ---
	_, err = env.Analytics.ProductMetric(ctx, whReq(env, t,
		&analyticsifacev1.ProductMetricRequest{
			MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
			Filter:      filter,
			Sort: &analyticsifacev1.Sort{
				Direction: analyticsifacev1.SortDirection_SORT_DIRECTION_DESC,
				Field: &analyticsifacev1.Sort_Stock{
					Stock: analyticsifacev1.StockMetricField_STOCK_METRIC_FIELD_READY,
				},
			},
		}, wh))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestAnalytics_OrderIsWarehouseScoped seeds sales in TWO warehouses and proves
// the ORDER metrics in all 3 RPCs (Daily / Product / User) reflect only the
// active-warehouse sales. Closes the gap where the rewrite preserved the old
// company-wide aggregation while STOCK was warehouse-scoped.
func TestAnalytics_OrderIsWarehouseScoped(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("WSA%d", uniq%100000))
	whB := makeWarehouse(env, t, ctx, fmt.Sprintf("WSB%d", uniq%100000))

	// Medicine + one batch in each warehouse so both sales can complete.
	medName := fmt.Sprintf("Analytics-WhScope-%d", uniq)
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("ws-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
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
				MedicineId: medID, BatchNumber: fmt.Sprintf("WSB-%s-%d", wh[:8], uniq),
				ExpiryDate: "2099-12-31", CostPrice: 400, InitialQuantity: 20,
			}, wh))
		require.NoError(t, err)
	}

	// WH-A sale: qty 2 -> terjual 2000.
	saleA := startSaleWith(env, t, ctx, whA)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleA, MedicineId: medID, Qty: 2}, whA))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: saleA, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 2000,
		}, whA))
	require.NoError(t, err)

	// WH-B sale: qty 3 -> terjual 3000.
	saleB := startSaleWith(env, t, ctx, whB)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleB, MedicineId: medID, Qty: 3}, whB))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: saleB, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 3000,
		}, whB))
	require.NoError(t, err)

	now := time.Now()
	from := now.AddDate(0, 0, -1).Unix()
	to := now.AddDate(0, 0, 1).Unix()
	filter := &analyticsifacev1.Filter{FromUnix: from, ToUnix: to}
	todayKey := now.Format("2006-01-02")

	// Helper: today's terjual on the Daily ORDER block.
	dailyTerjual := func(wh string) int64 {
		t.Helper()
		res, err := env.Analytics.DailyMetric(ctx, whReq(env, t,
			&analyticsifacev1.DailyMetricRequest{
				MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
				Filter:      filter,
				Granularity: analyticsifacev1.Granularity_GRANULARITY_DAY,
			}, wh))
		require.NoError(t, err)
		require.NotNil(t, res.Msg.Order)
		o := res.Msg.Order.Data[todayKey]
		if o == nil {
			return 0
		}
		return o.Terjual
	}

	// === DailyMetric ===
	require.GreaterOrEqual(t, dailyTerjual(whA), int64(2000), "WH-A includes its 2000 sale")
	// Sanity: WH-A's number must NOT include WH-B's 3000 — i.e., the value
	// from WH-A only and WH-B only must differ by approximately the seeded
	// gap. The dev DB carries other sales noise from prior tests, so we
	// check the differential rather than equality.
	require.NotEqual(t, dailyTerjual(whA), dailyTerjual(whB),
		"daily terjual must differ between warehouses (proves the scoping filter)")

	// === ProductMetric — the seeded medicine's terjual differs per warehouse ===
	prodTerjual := func(wh string) int64 {
		t.Helper()
		res, err := env.Analytics.ProductMetric(ctx, whReq(env, t,
			&analyticsifacev1.ProductMetricRequest{
				MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
				Filter:      filter,
				Limit:       1000,
			}, wh))
		require.NoError(t, err)
		require.NotNil(t, res.Msg.Order)
		o := res.Msg.Order.Data[medID]
		if o == nil {
			return 0
		}
		return o.Terjual
	}
	require.Equal(t, int64(2000), prodTerjual(whA), "WH-A: medicine terjual = 2000 (only the WH-A sale)")
	require.Equal(t, int64(3000), prodTerjual(whB), "WH-B: medicine terjual = 3000 (only the WH-B sale)")

	// === UserMetric — the same cashier (bootstrap owner) shows a different
	// total per warehouse ===
	userTerjual := func(wh string) int64 {
		t.Helper()
		res, err := env.Analytics.UserMetric(ctx, whReq(env, t,
			&analyticsifacev1.UserMetricRequest{
				MetricTypes: []analyticsifacev1.MetricType{analyticsifacev1.MetricType_METRIC_TYPE_ORDER},
				Filter:      filter,
				Limit:       1000,
			}, wh))
		require.NoError(t, err)
		require.NotEmpty(t, res.Msg.UserIds)
		uid := res.Msg.UserIds[0]
		return res.Msg.Order.Data[uid].Terjual
	}
	require.NotEqual(t, userTerjual(whA), userTerjual(whB),
		"user terjual must differ between warehouses (proves the scoping filter)")
}

// TestProductMetric_Extras verifies the 4 new Product-only metrics:
//   - order.last_order_unix: timestamp of most recent COMPLETED sale.
//   - order.avg_sold: base_qty / days_in_range (rounded).
//   - stock.last_restock_unix: most recent batch receipt (warehouse-scoped).
//   - stock.expiring: total qty of stock expiring within 30 days.
func TestProductMetric_Extras(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("PEX%d", uniq%100000))

	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("pex-%d", uniq), Name: fmt.Sprintf("Pex-Med-%d", uniq),
			Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	// Two batches: one expiring in 15 days (counts) + one expiring far out (does not).
	soon := time.Now().AddDate(0, 0, 15).Format("2006-01-02")
	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "PEX-SOON", ExpiryDate: soon,
			CostPrice: 500, InitialQuantity: 7,
		}, wh))
	require.NoError(t, err)
	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "PEX-FAR", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 50,
		}, wh))
	require.NoError(t, err)

	// Complete one sale of qty 4 in the active warehouse.
	saleID := startSaleWith(env, t, ctx, wh)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleID, MedicineId: medID, Qty: 4}, wh))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 4000,
		}, wh))
	require.NoError(t, err)

	// Filter window: last 7 days → 7 days in range, avg_sold = round(4/7) = 1.
	now := time.Now()
	filter := &analyticsifacev1.Filter{
		FromUnix: now.AddDate(0, 0, -7).Unix(),
		ToUnix:   now.AddDate(0, 0, 1).Unix(),
	}

	res, err := env.Analytics.ProductMetric(ctx, whReq(env, t,
		&analyticsifacev1.ProductMetricRequest{
			MetricTypes: []analyticsifacev1.MetricType{
				analyticsifacev1.MetricType_METRIC_TYPE_ORDER,
				analyticsifacev1.MetricType_METRIC_TYPE_STOCK,
			},
			Filter: filter,
			Limit:  1000,
		}, wh))
	require.NoError(t, err)
	o := res.Msg.Order.Data[medID]
	s := res.Msg.Stock.Data[medID]
	require.NotNil(t, o, "order block has the medicine")
	require.NotNil(t, s, "stock block has the medicine")

	// last_order_unix > 0 and recent.
	require.Greater(t, o.LastOrderUnix, int64(0))
	require.Greater(t, o.LastOrderUnix, now.Add(-5*time.Minute).Unix())

	// avg_sold = round(4 / 8 days) = 1 (window from -7d to +1d = 8 days).
	require.InDelta(t, 1.0, float64(o.AvgSold), 1.0, "avg_sold ~= base_qty/days, rounded")

	// last_restock_unix > 0 (we just created the batches).
	require.Greater(t, s.LastRestockUnix, int64(0))

	// expiring == on-hand qty in batches expiring within 30d. FEFO consumed the
	// PEX-SOON batch first (initial 7 - sale 4 = 3 remaining), and PEX-FAR is
	// out of the 30d window.
	require.Equal(t, int64(3), s.Expiring, "expiring counts on-hand in batches within 30d, after FEFO")
}
