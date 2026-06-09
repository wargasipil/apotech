//go:build !sqlite

package service

import (
	"context"
	"time"

	analyticsifacev1 "github.com/apotech/backend/gen/analytics_iface/v1"
)

// dailyOrderMetric (Postgres flavor): DATE_TRUNC returns a TIMESTAMP that GORM
// scans straight into a time.Time field, which the Go-side `dayBucketKey`
// then formats to "YYYY-MM-DD" / "YYYY-Www" / "YYYY-MM" in the Go-local
// timezone — matching the bucket keys the frontend expects.
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
