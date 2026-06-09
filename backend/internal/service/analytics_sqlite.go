//go:build sqlite

package service

import (
	"context"
	"time"

	analyticsifacev1 "github.com/apotech/backend/gen/analytics_iface/v1"
)

// dailyOrderMetric (SQLite flavor): strftime produces the bucket-key STRING
// directly (SQLite has no DATE_TRUNC and stores DATETIME as text, so a
// timestamp-typed scan would round-trip through Go's time parser). We then
// parse it back to time.Time so the same `dayBucketKey` helper formats the
// final response key — keeping the response format identical to the Postgres
// flavor.
//
// strftime uses UTC by default for parsed text-stored timestamps; if the
// portable build's local timezone differs from UTC, bucket boundaries may
// shift by up to one day at midnight. Acceptable for single-shop deployments;
// document if it bites.
func (a *Analytics) dailyOrderMetric(ctx context.Context, from, to time.Time, gran, warehouseID string) (map[string]*analyticsifacev1.OrderItem, error) {
	type row struct {
		Bucket  string `gorm:"column:bucket"`
		Terjual int64
		Hpp     int64
	}
	var rows []row
	err := a.db.WithContext(ctx).Raw(`
		SELECT strftime('%Y-%m-%d', s.completed_at) AS bucket,
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
	`, saleStatusCompleted, warehouseID, from, to).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]*analyticsifacev1.OrderItem{}
	for _, r := range rows {
		// Parse the strftime output back to a time.Time so dayBucketKey can
		// produce the same week/month rollups Postgres does.
		t, perr := time.ParseInLocation("2006-01-02", r.Bucket, time.Local)
		if perr != nil {
			continue
		}
		out[dayBucketKey(t, gran)] = &analyticsifacev1.OrderItem{
			Terjual: r.Terjual,
			Hpp:     r.Hpp,
			Profit:  r.Terjual - r.Hpp,
		}
	}
	return out, nil
}
