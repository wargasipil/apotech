package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	analyticsifacev1 "github.com/apotech/backend/gen/analytics_iface/v1"
	"github.com/apotech/backend/internal/auth"
)

// Analytics implements analytics_iface.v1.AnalyticsService — three
// dimension-scoped RPCs (DailyMetric, ProductMetric, UserMetric). Pattern:
// phase 1 = sort + paginate to get IDs; phase 2 = fetch metric blocks for
// those IDs. Map-keyed response + separate sorted `ids` list, so the
// frontend resolves names via Resolve<Domain>(ids) and the metric handler
// stays N+1-free.
type Analytics struct {
	db *gorm.DB
}

func NewAnalytics(db *gorm.DB) *Analytics { return &Analytics{db: db} }

// ---------- shared helpers ----------

// analyticsDateRange defaults to the last 30 days when either bound is unset.
// Mirrors the pattern in the deleted analytics_sales.go dateRangeOrDefault.
func analyticsDateRange(f *analyticsifacev1.Filter) (time.Time, time.Time) {
	now := time.Now()
	var from, to time.Time
	if f != nil && f.ToUnix > 0 {
		to = time.Unix(f.ToUnix, 0)
	} else {
		to = now
	}
	if f != nil && f.FromUnix > 0 {
		from = time.Unix(f.FromUnix, 0)
	} else {
		from = to.AddDate(0, 0, -30)
	}
	return from, to
}

// truncFmt maps Granularity to the DATE_TRUNC string Postgres accepts.
func truncFmt(g analyticsifacev1.Granularity) string {
	switch g {
	case analyticsifacev1.Granularity_GRANULARITY_MONTH:
		return "month"
	case analyticsifacev1.Granularity_GRANULARITY_WEEK:
		return "week"
	default:
		return "day"
	}
}

// enumerateBuckets walks [from, to) in granularity-sized steps in local time,
// returning the bucket-start timestamp for each step. Each start is normalized
// to the bucket boundary (Monday for week, 1st of month for month).
func enumerateBuckets(from, to time.Time, granularity string) []time.Time {
	out := []time.Time{}
	cur := bucketStart(from, granularity)
	for cur.Before(to) {
		out = append(out, cur)
		cur = bucketNext(cur, granularity)
	}
	return out
}

// bucketStart truncates t to the start of its bucket.
func bucketStart(t time.Time, granularity string) time.Time {
	loc := t.Location()
	switch granularity {
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	case "week":
		// ISO week starts Monday. Go's Weekday: Sunday=0..Saturday=6.
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		return d.AddDate(0, 0, -(wd - 1))
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}
}

// bucketNext returns the next bucket's start (used for both iteration and the
// as-of-end-of-bucket boundary in stock reconstruction).
func bucketNext(t time.Time, granularity string) time.Time {
	switch granularity {
	case "month":
		return t.AddDate(0, 1, 0)
	case "week":
		return t.AddDate(0, 0, 7)
	default:
		return t.AddDate(0, 0, 1)
	}
}

// dayBucketKey formats a bucket boundary timestamp into the response key:
//
//	day   = "YYYY-MM-DD"
//	week  = "YYYY-Www" (ISO week)
//	month = "YYYY-MM"
func dayBucketKey(t time.Time, granularity string) string {
	switch granularity {
	case "month":
		return t.Format("2006-01")
	case "week":
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	default:
		return t.Format("2006-01-02")
	}
}

// containsMetric reports whether the metric types list includes the given
// MetricType. Treats _UNSPECIFIED as "not present".
func containsMetric(types []analyticsifacev1.MetricType, want analyticsifacev1.MetricType) bool {
	for _, m := range types {
		if m == want {
			return true
		}
	}
	return false
}

// validateMetricTypes rejects duplicates, _UNSPECIFIED, and (when forbid is
// set) the forbidden metric type. Returns a connect.InvalidArgument error.
func validateMetricTypes(types []analyticsifacev1.MetricType, forbid analyticsifacev1.MetricType, forbidMsg string) error {
	if len(types) == 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("metric_types must not be empty"))
	}
	seen := map[analyticsifacev1.MetricType]bool{}
	for _, m := range types {
		if m == analyticsifacev1.MetricType_METRIC_TYPE_UNSPECIFIED {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("metric_types must not contain UNSPECIFIED"))
		}
		if seen[m] {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("metric_types must not contain duplicates"))
		}
		seen[m] = true
		if forbid != analyticsifacev1.MetricType_METRIC_TYPE_UNSPECIFIED && m == forbid {
			return connect.NewError(connect.CodeInvalidArgument, errors.New(forbidMsg))
		}
	}
	return nil
}

// validateSort enforces "Sort.field must reference a requested metric type".
// Returns nil when sort is unset (= sort by dimension key).
func validateSort(sort *analyticsifacev1.Sort, types []analyticsifacev1.MetricType, forbidStock bool) error {
	if sort == nil {
		return nil
	}
	switch sort.Field.(type) {
	case *analyticsifacev1.Sort_Order:
		if !containsMetric(types, analyticsifacev1.MetricType_METRIC_TYPE_ORDER) {
			return connect.NewError(connect.CodeInvalidArgument,
				errors.New("sort field references ORDER but metric_types does not include METRIC_TYPE_ORDER"))
		}
	case *analyticsifacev1.Sort_Stock:
		if forbidStock {
			return connect.NewError(connect.CodeInvalidArgument,
				errors.New("stock sort not supported on user dimension"))
		}
		if !containsMetric(types, analyticsifacev1.MetricType_METRIC_TYPE_STOCK) {
			return connect.NewError(connect.CodeInvalidArgument,
				errors.New("sort field references STOCK but metric_types does not include METRIC_TYPE_STOCK"))
		}
	}
	return nil
}

// sortDirSQL maps SortDirection to "ASC"/"DESC" (default ASC).
func sortDirSQL(d analyticsifacev1.SortDirection) string {
	if d == analyticsifacev1.SortDirection_SORT_DIRECTION_DESC {
		return "DESC"
	}
	return "ASC"
}

// orderMetricColumn maps an OrderMetricField to its SQL column alias in the
// phase-1/phase-2 aggregations.
func orderMetricColumn(f analyticsifacev1.OrderMetricField) string {
	switch f {
	case analyticsifacev1.OrderMetricField_ORDER_METRIC_FIELD_HPP:
		return "hpp"
	case analyticsifacev1.OrderMetricField_ORDER_METRIC_FIELD_PROFIT:
		return "profit"
	default:
		return "terjual"
	}
}

func stockMetricColumn(f analyticsifacev1.StockMetricField) string {
	if f == analyticsifacev1.StockMetricField_STOCK_METRIC_FIELD_ONGOING {
		return "ongoing"
	}
	return "ready"
}

// ---------- DailyMetric ----------

func (a *Analytics) DailyMetric(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.DailyMetricRequest],
) (*connect.Response[analyticsifacev1.DailyMetricResponse], error) {
	if err := validateMetricTypes(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_UNSPECIFIED, ""); err != nil {
		return nil, err
	}
	if err := validateSort(req.Msg.Sort, req.Msg.MetricTypes, false); err != nil {
		return nil, err
	}

	from, to := analyticsDateRange(req.Msg.Filter)
	gran := truncFmt(req.Msg.Granularity)
	wantOrder := containsMetric(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_ORDER)
	wantStock := containsMetric(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_STOCK)

	// Resolve the active warehouse unconditionally. ORDER and STOCK both need
	// it now (sales carry warehouse_id; the filter brings analytics in line
	// with the rest of the app's active-warehouse convention).
	caller, perr := auth.MustPrincipal(ctx)
	if perr != nil {
		return nil, perr
	}
	warehouseID, werr := resolveWarehouse(ctx, a.db, caller)
	if werr != nil {
		return nil, connect.NewError(connect.CodeInternal, werr)
	}

	// Phase 1 — enumerate bucket keys in [from, to) in Go. Each bucket start is
	// the canonical boundary for the chosen granularity. Simpler + portable
	// than Postgres generate_series with text intervals.
	buckets := enumerateBuckets(from, to, gran)
	if len(buckets) == 0 {
		return connect.NewResponse(&analyticsifacev1.DailyMetricResponse{}), nil
	}
	bucketKeys := make([]string, 0, len(buckets))
	for _, b := range buckets {
		bucketKeys = append(bucketKeys, dayBucketKey(b, gran))
	}
	var err error

	// Phase 2a — ORDER metric per bucket.
	orderByKey := map[string]*analyticsifacev1.OrderItem{}
	if wantOrder {
		orderByKey, err = a.dailyOrderMetric(ctx, from, to, gran, warehouseID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Phase 2b — STOCK metric per bucket (as-of-end-of-day reconstruction).
	stockByKey := map[string]*analyticsifacev1.StockItem{}
	if wantStock {
		stockByKey, err = a.dailyStockMetric(ctx, warehouseID, buckets, bucketKeys, gran)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Sort the day keys per Sort.field (default chronological asc).
	dayOrder := sortDailyKeys(bucketKeys, req.Msg.Sort, orderByKey, stockByKey)

	out := &analyticsifacev1.DailyMetricResponse{Days: dayOrder}
	if wantOrder {
		out.Order = &analyticsifacev1.MetricOrder{Data: ensureOrderForKeys(orderByKey, dayOrder)}
	}
	if wantStock {
		out.Stock = &analyticsifacev1.MetricStock{Data: ensureStockForKeys(stockByKey, dayOrder)}
	}
	return connect.NewResponse(out), nil
}

func (a *Analytics) dailyOrderMetric(ctx context.Context, from, to time.Time, gran, warehouseID string) (map[string]*analyticsifacev1.OrderItem, error) {
	type row struct {
		Bucket  time.Time `gorm:"column:bucket"`
		Terjual int64
		Hpp     int64
	}
	var rows []row
	err := a.db.WithContext(ctx).Raw(`
		SELECT DATE_TRUNC(?, s.completed_at) AS bucket,
		       COALESCE(SUM(si.line_total), 0) AS terjual,
		       COALESCE(SUM(c.cogs), 0)        AS hpp
		FROM sales s
		JOIN sale_items si ON si.sale_id = s.id
		LEFT JOIN (
		  SELECT sm.sale_item_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
		  FROM stock_movements sm
		  JOIN batches b ON b.id = sm.batch_id
		  WHERE sm.type = 'SALE' AND sm.sale_item_id IS NOT NULL
		  GROUP BY sm.sale_item_id
		) c ON c.sale_item_id = si.id
		WHERE s.status = ? AND s.warehouse_id = ?
		  AND s.completed_at >= ? AND s.completed_at < ?
		GROUP BY bucket
	`, gran, saleStatusCompleted, warehouseID, from, to).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]*analyticsifacev1.OrderItem{}
	for _, r := range rows {
		out[dayBucketKey(r.Bucket, gran)] = &analyticsifacev1.OrderItem{
			Terjual: r.Terjual,
			Hpp:     r.Hpp,
			Profit:  r.Terjual - r.Hpp,
		}
	}
	return out, nil
}

func (a *Analytics) dailyStockMetric(
	ctx context.Context,
	warehouseID string,
	buckets []time.Time,
	bucketKeys []string,
	gran string,
) (map[string]*analyticsifacev1.StockItem, error) {
	// `ongoing` is a current snapshot reused on every row (PO data is
	// forward-looking; historical reconstruction of in-flight POs is out of
	// scope — documented in the plan).
	var ongoing int64
	if err := a.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(ordered_qty - received_qty), 0)
		FROM purchase_order_items poi
		JOIN purchase_orders po ON po.id = poi.purchase_order_id
		WHERE po.status NOT IN ('VOIDED', 'CLOSED', 'RECEIVED')
	`).Scan(&ongoing).Error; err != nil {
		return nil, err
	}

	out := map[string]*analyticsifacev1.StockItem{}
	for i, b := range buckets {
		// As-of-end-of-bucket: stock = SUM(qty) of movements created before the
		// NEXT bucket start. We bucket the boundary at DATE_TRUNC(gran, b) +
		// 1*gran, which is the next bucket's start.
		var ready int64
		var bucketEnd time.Time
		switch gran {
		case "month":
			bucketEnd = b.AddDate(0, 1, 0)
		case "week":
			bucketEnd = b.AddDate(0, 0, 7)
		default:
			bucketEnd = b.AddDate(0, 0, 1)
		}
		if err := a.db.WithContext(ctx).Raw(`
			SELECT COALESCE(SUM(qty), 0)
			FROM stock_movements
			WHERE warehouse_id = ? AND created_at < ?
		`, warehouseID, bucketEnd).Scan(&ready).Error; err != nil {
			return nil, err
		}
		out[bucketKeys[i]] = &analyticsifacev1.StockItem{
			Ready:   ready,
			Ongoing: ongoing,
		}
	}
	return out, nil
}

// sortDailyKeys returns the day-keys in the order the UI should render. When
// sort references a metric, sort by that metric's value (missing keys get 0);
// otherwise chronological asc.
func sortDailyKeys(
	keys []string,
	sort *analyticsifacev1.Sort,
	order map[string]*analyticsifacev1.OrderItem,
	stock map[string]*analyticsifacev1.StockItem,
) []string {
	if sort == nil || sort.Field == nil {
		// keys are already chronological asc from generate_series
		return keys
	}
	out := make([]string, len(keys))
	copy(out, keys)
	desc := sort.Direction == analyticsifacev1.SortDirection_SORT_DIRECTION_DESC
	switch f := sort.Field.(type) {
	case *analyticsifacev1.Sort_Order:
		col := f.Order
		sortInPlace(out, desc, func(k string) int64 {
			o := order[k]
			if o == nil {
				return 0
			}
			switch col {
			case analyticsifacev1.OrderMetricField_ORDER_METRIC_FIELD_HPP:
				return o.Hpp
			case analyticsifacev1.OrderMetricField_ORDER_METRIC_FIELD_PROFIT:
				return o.Profit
			default:
				return o.Terjual
			}
		})
	case *analyticsifacev1.Sort_Stock:
		col := f.Stock
		sortInPlace(out, desc, func(k string) int64 {
			s := stock[k]
			if s == nil {
				return 0
			}
			if col == analyticsifacev1.StockMetricField_STOCK_METRIC_FIELD_ONGOING {
				return s.Ongoing
			}
			return s.Ready
		})
	}
	return out
}

// sortInPlace sorts the string slice by the int64-valued accessor, asc or desc.
func sortInPlace(xs []string, desc bool, val func(string) int64) {
	// Small N (days bounded by range), so a simple insertion sort is fine and
	// keeps deps minimal.
	for i := 1; i < len(xs); i++ {
		x := xs[i]
		vx := val(x)
		j := i - 1
		for j >= 0 {
			cmp := val(xs[j]) < vx
			if desc {
				cmp = val(xs[j]) > vx
			}
			if cmp {
				break
			}
			xs[j+1] = xs[j]
			j--
		}
		xs[j+1] = x
	}
}

// ensureOrderForKeys/ensureStockForKeys make sure every key in the response's
// ordering list has an entry in the map (zero-valued when missing). Keeps the
// frontend's `order.data[id]` lookup safe.
func ensureOrderForKeys(m map[string]*analyticsifacev1.OrderItem, keys []string) map[string]*analyticsifacev1.OrderItem {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			m[k] = &analyticsifacev1.OrderItem{}
		}
	}
	return m
}
func ensureStockForKeys(m map[string]*analyticsifacev1.StockItem, keys []string) map[string]*analyticsifacev1.StockItem {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			m[k] = &analyticsifacev1.StockItem{}
		}
	}
	return m
}

// ---------- ProductMetric ----------

func (a *Analytics) ProductMetric(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.ProductMetricRequest],
) (*connect.Response[analyticsifacev1.ProductMetricResponse], error) {
	if err := validateMetricTypes(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_UNSPECIFIED, ""); err != nil {
		return nil, err
	}
	if err := validateSort(req.Msg.Sort, req.Msg.MetricTypes, false); err != nil {
		return nil, err
	}

	from, to := analyticsDateRange(req.Msg.Filter)
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	wantOrder := containsMetric(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_ORDER)
	wantStock := containsMetric(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_STOCK)

	caller, perr := auth.MustPrincipal(ctx)
	if perr != nil {
		return nil, perr
	}
	warehouseID, werr := resolveWarehouse(ctx, a.db, caller)
	if werr != nil {
		return nil, connect.NewError(connect.CodeInternal, werr)
	}

	// Phase 1 — sort + page ids. We build a combined CTE so the sort metric
	// has a column to ORDER BY regardless of which dimensions feed it.
	ids, total, err := a.productPageIDs(ctx, from, to, warehouseID, req.Msg.Sort, wantOrder, wantStock, limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := &analyticsifacev1.ProductMetricResponse{
		MedicineIds: ids,
		Total:       int32(total),
	}
	if len(ids) == 0 {
		return connect.NewResponse(out), nil
	}

	// Phase 2 — metric blocks for the page ids only.
	if wantOrder {
		om, err := a.productOrderForIDs(ctx, from, to, warehouseID, ids)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out.Order = &analyticsifacev1.MetricOrder{Data: ensureOrderForKeys(om, ids)}
	}
	if wantStock {
		sm, err := a.productStockForIDs(ctx, warehouseID, ids)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out.Stock = &analyticsifacev1.MetricStock{Data: ensureStockForKeys(sm, ids)}
	}
	return connect.NewResponse(out), nil
}

// productPageIDs builds the universe of candidate medicine ids (union of any
// medicine with an order in range OR any medicine with stock in the active
// warehouse), then sorts + paginates.
func (a *Analytics) productPageIDs(
	ctx context.Context,
	from, to time.Time,
	warehouseID string,
	sort *analyticsifacev1.Sort,
	wantOrder, wantStock bool,
	limit, offset int,
) ([]string, int64, error) {
	var orderClause string
	if sort != nil {
		dir := sortDirSQL(sort.Direction)
		switch f := sort.Field.(type) {
		case *analyticsifacev1.Sort_Order:
			orderClause = fmt.Sprintf("ORDER BY %s %s NULLS LAST, m.name ASC", orderMetricColumn(f.Order), dir)
		case *analyticsifacev1.Sort_Stock:
			orderClause = fmt.Sprintf("ORDER BY %s %s NULLS LAST, m.name ASC", stockMetricColumn(f.Stock), dir)
		}
	}
	if orderClause == "" {
		orderClause = "ORDER BY m.name ASC"
	}

	// Combined CTE — compute every metric the sort might reference so the
	// ORDER BY can name them by alias. Total = COUNT(*) over the inner CTE.
	combined := `
		WITH order_agg AS (
		  SELECT si.medicine_id,
		         SUM(si.line_total) AS terjual,
		         COALESCE(SUM(c.cogs), 0) AS hpp
		  FROM sale_items si
		  JOIN sales s ON s.id = si.sale_id
		  LEFT JOIN (
		    SELECT sm.sale_item_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
		    FROM stock_movements sm
		    JOIN batches b ON b.id = sm.batch_id
		    WHERE sm.type = 'SALE' AND sm.sale_item_id IS NOT NULL
		    GROUP BY sm.sale_item_id
		  ) c ON c.sale_item_id = si.id
		  WHERE s.status = ? AND s.warehouse_id = ?
		    AND s.completed_at >= ? AND s.completed_at < ?
		  GROUP BY si.medicine_id
		),
		stock_agg AS (
		  SELECT b.medicine_id,
		         COALESCE(SUM(sm.qty), 0) AS ready
		  FROM batches b
		  LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?
		  GROUP BY b.medicine_id
		),
		ongoing_agg AS (
		  SELECT poi.medicine_id, COALESCE(SUM(poi.ordered_qty - poi.received_qty), 0) AS ongoing
		  FROM purchase_order_items poi
		  JOIN purchase_orders po ON po.id = poi.purchase_order_id
		  WHERE po.status NOT IN ('VOIDED', 'CLOSED', 'RECEIVED')
		  GROUP BY poi.medicine_id
		),
		combined AS (
		  SELECT m.id, m.name,
		         COALESCE(o.terjual, 0) AS terjual,
		         COALESCE(o.hpp, 0) AS hpp,
		         (COALESCE(o.terjual, 0) - COALESCE(o.hpp, 0)) AS profit,
		         COALESCE(s.ready, 0) AS ready,
		         COALESCE(g.ongoing, 0) AS ongoing
		  FROM medicines m
		  LEFT JOIN order_agg o   ON o.medicine_id = m.id
		  LEFT JOIN stock_agg s   ON s.medicine_id = m.id
		  LEFT JOIN ongoing_agg g ON g.medicine_id = m.id
		  WHERE m.active = true
		)
	`

	// Param order: order_agg uses (status, warehouseID, from, to);
	// stock_agg uses (warehouseID). 5 total positional params.
	var total int64
	if err := a.db.WithContext(ctx).Raw(
		combined+`SELECT COUNT(*) FROM combined`,
		saleStatusCompleted, warehouseID, from, to, warehouseID,
	).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	type idRow struct{ ID string }
	var rows []idRow
	err := a.db.WithContext(ctx).Raw(
		combined+`SELECT id FROM combined m `+orderClause+` LIMIT ? OFFSET ?`,
		saleStatusCompleted, warehouseID, from, to, warehouseID, limit, offset,
	).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids, total, nil
}

func (a *Analytics) productOrderForIDs(ctx context.Context, from, to time.Time, warehouseID string, ids []string) (map[string]*analyticsifacev1.OrderItem, error) {
	type row struct {
		MedicineID string `gorm:"column:medicine_id"`
		Terjual    int64
		Hpp        int64
	}
	var rows []row
	err := a.db.WithContext(ctx).Raw(`
		SELECT si.medicine_id,
		       COALESCE(SUM(si.line_total), 0) AS terjual,
		       COALESCE(SUM(c.cogs), 0) AS hpp
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		LEFT JOIN (
		  SELECT sm.sale_item_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
		  FROM stock_movements sm
		  JOIN batches b ON b.id = sm.batch_id
		  WHERE sm.type = 'SALE' AND sm.sale_item_id IS NOT NULL
		  GROUP BY sm.sale_item_id
		) c ON c.sale_item_id = si.id
		WHERE s.status = ? AND s.warehouse_id = ?
		  AND s.completed_at >= ? AND s.completed_at < ?
		  AND si.medicine_id IN ?
		GROUP BY si.medicine_id
	`, saleStatusCompleted, warehouseID, from, to, ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]*analyticsifacev1.OrderItem{}
	for _, r := range rows {
		out[r.MedicineID] = &analyticsifacev1.OrderItem{
			Terjual: r.Terjual,
			Hpp:     r.Hpp,
			Profit:  r.Terjual - r.Hpp,
		}
	}
	return out, nil
}

func (a *Analytics) productStockForIDs(ctx context.Context, warehouseID string, ids []string) (map[string]*analyticsifacev1.StockItem, error) {
	type row struct {
		MedicineID string `gorm:"column:medicine_id"`
		Ready      int64
		Ongoing    int64
	}
	var rows []row
	err := a.db.WithContext(ctx).Raw(`
		SELECT m.id AS medicine_id,
		       COALESCE(s.ready, 0)   AS ready,
		       COALESCE(g.ongoing, 0) AS ongoing
		FROM medicines m
		LEFT JOIN (
		  SELECT b.medicine_id, COALESCE(SUM(sm.qty), 0) AS ready
		  FROM batches b
		  LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?
		  GROUP BY b.medicine_id
		) s ON s.medicine_id = m.id
		LEFT JOIN (
		  SELECT poi.medicine_id, COALESCE(SUM(poi.ordered_qty - poi.received_qty), 0) AS ongoing
		  FROM purchase_order_items poi
		  JOIN purchase_orders po ON po.id = poi.purchase_order_id
		  WHERE po.status NOT IN ('VOIDED', 'CLOSED', 'RECEIVED')
		  GROUP BY poi.medicine_id
		) g ON g.medicine_id = m.id
		WHERE m.id IN ?
	`, warehouseID, ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]*analyticsifacev1.StockItem{}
	for _, r := range rows {
		out[r.MedicineID] = &analyticsifacev1.StockItem{
			Ready:   r.Ready,
			Ongoing: r.Ongoing,
		}
	}
	return out, nil
}

// ---------- UserMetric ----------

func (a *Analytics) UserMetric(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.UserMetricRequest],
) (*connect.Response[analyticsifacev1.UserMetricResponse], error) {
	// USER dimension rejects STOCK in any form (metric_types or sort.field).
	if err := validateMetricTypes(req.Msg.MetricTypes,
		analyticsifacev1.MetricType_METRIC_TYPE_STOCK,
		"stock metric not supported on user dimension"); err != nil {
		return nil, err
	}
	if err := validateSort(req.Msg.Sort, req.Msg.MetricTypes, true); err != nil {
		return nil, err
	}

	from, to := analyticsDateRange(req.Msg.Filter)
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	wantOrder := containsMetric(req.Msg.MetricTypes, analyticsifacev1.MetricType_METRIC_TYPE_ORDER)

	// Scope ORDER metrics to the active warehouse — matches Daily/Product and
	// the rest of the app's convention. Sales carry warehouse_id from
	// StartSale.
	caller, perr := auth.MustPrincipal(ctx)
	if perr != nil {
		return nil, perr
	}
	warehouseID, werr := resolveWarehouse(ctx, a.db, caller)
	if werr != nil {
		return nil, connect.NewError(connect.CodeInternal, werr)
	}

	var orderClause string
	if req.Msg.Sort != nil {
		if f, ok := req.Msg.Sort.Field.(*analyticsifacev1.Sort_Order); ok {
			orderClause = fmt.Sprintf("ORDER BY %s %s NULLS LAST, u.name ASC",
				orderMetricColumn(f.Order), sortDirSQL(req.Msg.Sort.Direction))
		}
	}
	if orderClause == "" {
		orderClause = "ORDER BY u.name ASC"
	}

	combined := `
		WITH order_agg AS (
		  SELECT s.cashier_user_id AS user_id,
		         COALESCE(SUM(s.total), 0) AS terjual,
		         COALESCE(SUM(c.cogs), 0) AS hpp
		  FROM sales s
		  LEFT JOIN (
		    SELECT si.sale_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
		    FROM sale_items si
		    JOIN stock_movements sm ON sm.sale_item_id = si.id AND sm.type = 'SALE'
		    JOIN batches b ON b.id = sm.batch_id
		    GROUP BY si.sale_id
		  ) c ON c.sale_id = s.id
		  WHERE s.status = ? AND s.warehouse_id = ?
		    AND s.completed_at >= ? AND s.completed_at < ?
		  GROUP BY s.cashier_user_id
		),
		combined AS (
		  SELECT u.id, u.name,
		         COALESCE(o.terjual, 0) AS terjual,
		         COALESCE(o.hpp, 0) AS hpp,
		         (COALESCE(o.terjual, 0) - COALESCE(o.hpp, 0)) AS profit
		  FROM users u
		  JOIN order_agg o ON o.user_id = u.id
		  WHERE u.active = true
		)
	`

	var total int64
	if err := a.db.WithContext(ctx).Raw(
		combined+`SELECT COUNT(*) FROM combined`,
		saleStatusCompleted, warehouseID, from, to,
	).Scan(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type idRow struct{ ID string }
	var rows []idRow
	if err := a.db.WithContext(ctx).Raw(
		combined+`SELECT id FROM combined u `+orderClause+` LIMIT ? OFFSET ?`,
		saleStatusCompleted, warehouseID, from, to, limit, offset,
	).Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}

	out := &analyticsifacev1.UserMetricResponse{
		UserIds: ids,
		Total:   int32(total),
	}
	if len(ids) == 0 {
		return connect.NewResponse(out), nil
	}

	if wantOrder {
		type orow struct {
			UserID  string `gorm:"column:user_id"`
			Terjual int64
			Hpp     int64
		}
		var orows []orow
		err := a.db.WithContext(ctx).Raw(`
			SELECT s.cashier_user_id AS user_id,
			       COALESCE(SUM(s.total), 0) AS terjual,
			       COALESCE(SUM(c.cogs), 0) AS hpp
			FROM sales s
			LEFT JOIN (
			  SELECT si.sale_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
			  FROM sale_items si
			  JOIN stock_movements sm ON sm.sale_item_id = si.id AND sm.type = 'SALE'
			  JOIN batches b ON b.id = sm.batch_id
			  GROUP BY si.sale_id
			) c ON c.sale_id = s.id
			WHERE s.status = ? AND s.warehouse_id = ?
			  AND s.completed_at >= ? AND s.completed_at < ?
			  AND s.cashier_user_id IN ?
			GROUP BY s.cashier_user_id
		`, saleStatusCompleted, warehouseID, from, to, ids).Scan(&orows).Error
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		m := map[string]*analyticsifacev1.OrderItem{}
		for _, r := range orows {
			m[r.UserID] = &analyticsifacev1.OrderItem{
				Terjual: r.Terjual,
				Hpp:     r.Hpp,
				Profit:  r.Terjual - r.Hpp,
			}
		}
		out.Order = &analyticsifacev1.MetricOrder{Data: ensureOrderForKeys(m, ids)}
	}
	return connect.NewResponse(out), nil
}
