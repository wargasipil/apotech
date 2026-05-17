package service

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	analyticsifacev1 "github.com/apotech/backend/gen/analytics_iface/v1"
)

type InventoryAnalytics struct {
	db *gorm.DB
}

func NewInventoryAnalytics(db *gorm.DB) *InventoryAnalytics { return &InventoryAnalytics{db: db} }

func (a *InventoryAnalytics) GetTurnover(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetTurnoverRequest],
) (*connect.Response[analyticsifacev1.GetTurnoverResponse], error) {
	periodDays := int(req.Msg.PeriodDays)
	if periodDays <= 0 {
		periodDays = 30
	}
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	from := time.Now().AddDate(0, 0, -periodDays)

	// Turnover ratio = sold_qty / avg_inventory_qty over the period.
	// sold_qty: sum of -qty for SALE movements in window.
	// avg_inventory_qty: average current stock at period_end and period_start,
	//   approximated as current_stock + (sales_during_window/2)
	//   (Simple model: ending_stock + half of what was sold.)
	type row struct {
		MedicineID      string `gorm:"column:medicine_id"`
		MedicineName    string `gorm:"column:medicine_name"`
		SKU             string `gorm:"column:sku"`
		SoldQty         int64  `gorm:"column:sold_qty"`
		AvgInventoryQty int64  `gorm:"column:avg_inventory_qty"`
		TurnoverRatio   float64 `gorm:"column:turnover_ratio"`
	}
	var rows []row
	err := a.db.WithContext(ctx).Raw(`
		WITH window_sales AS (
			SELECT b.medicine_id, COALESCE(SUM(-sm.qty), 0) AS sold_qty
			FROM stock_movements sm
			JOIN batches b ON b.id = sm.batch_id
			WHERE sm.type = 'SALE' AND sm.created_at >= ?
			GROUP BY b.medicine_id
		),
		current_stock AS (
			SELECT b.medicine_id, COALESCE(SUM(sm.qty), 0) AS current_qty
			FROM batches b
			LEFT JOIN stock_movements sm ON sm.batch_id = b.id
			GROUP BY b.medicine_id
		)
		SELECT m.id AS medicine_id,
		       m.name AS medicine_name,
		       m.sku,
		       COALESCE(ws.sold_qty, 0) AS sold_qty,
		       GREATEST(COALESCE(cs.current_qty, 0) + COALESCE(ws.sold_qty, 0) / 2, 1) AS avg_inventory_qty,
		       COALESCE(ws.sold_qty, 0)::float
		         / GREATEST(COALESCE(cs.current_qty, 0) + COALESCE(ws.sold_qty, 0) / 2, 1) AS turnover_ratio
		FROM medicines m
		LEFT JOIN window_sales ws ON ws.medicine_id = m.id
		LEFT JOIN current_stock cs ON cs.medicine_id = m.id
		WHERE m.active = TRUE
		ORDER BY turnover_ratio DESC
		LIMIT ?
	`, from, limit).Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*analyticsifacev1.TurnoverRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, &analyticsifacev1.TurnoverRow{
			MedicineId:      r.MedicineID,
			MedicineName:    r.MedicineName,
			Sku:             r.SKU,
			SoldQty:         r.SoldQty,
			AvgInventoryQty: r.AvgInventoryQty,
			TurnoverRatio:   r.TurnoverRatio,
		})
	}
	return connect.NewResponse(&analyticsifacev1.GetTurnoverResponse{Rows: out}), nil
}

func (a *InventoryAnalytics) GetDeadStock(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetDeadStockRequest],
) (*connect.Response[analyticsifacev1.GetDeadStockResponse], error) {
	noMov := int(req.Msg.NoMovementDays)
	if noMov <= 0 {
		noMov = 60
	}
	cutoff := time.Now().AddDate(0, 0, -noMov)

	type row struct {
		MedicineID   string     `gorm:"column:medicine_id"`
		MedicineName string     `gorm:"column:medicine_name"`
		SKU          string     `gorm:"column:sku"`
		CurrentQty   int64      `gorm:"column:current_qty"`
		LastSale     *time.Time `gorm:"column:last_sale"`
	}
	var rows []row
	err := a.db.WithContext(ctx).Raw(`
		WITH current_stock AS (
			SELECT b.medicine_id, COALESCE(SUM(sm.qty), 0) AS current_qty
			FROM batches b
			LEFT JOIN stock_movements sm ON sm.batch_id = b.id
			GROUP BY b.medicine_id
		),
		last_sale AS (
			SELECT b.medicine_id, MAX(sm.created_at) AS last_sale
			FROM stock_movements sm
			JOIN batches b ON b.id = sm.batch_id
			WHERE sm.type = 'SALE'
			GROUP BY b.medicine_id
		)
		SELECT m.id AS medicine_id, m.name AS medicine_name, m.sku,
		       cs.current_qty, ls.last_sale
		FROM medicines m
		JOIN current_stock cs ON cs.medicine_id = m.id
		LEFT JOIN last_sale ls ON ls.medicine_id = m.id
		WHERE m.active = TRUE
		  AND cs.current_qty > 0
		  AND (ls.last_sale IS NULL OR ls.last_sale < ?)
		ORDER BY cs.current_qty DESC
	`, cutoff).Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*analyticsifacev1.DeadStockRow, 0, len(rows))
	for _, r := range rows {
		var lastSaleUnix int64
		if r.LastSale != nil {
			lastSaleUnix = r.LastSale.Unix()
		}
		out = append(out, &analyticsifacev1.DeadStockRow{
			MedicineId:   r.MedicineID,
			MedicineName: r.MedicineName,
			Sku:          r.SKU,
			CurrentQty:   r.CurrentQty,
			LastSaleUnix: lastSaleUnix,
		})
	}
	return connect.NewResponse(&analyticsifacev1.GetDeadStockResponse{Rows: out}), nil
}

func (a *InventoryAnalytics) GetDaysOfStockRemaining(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetDaysOfStockRemainingRequest],
) (*connect.Response[analyticsifacev1.GetDaysOfStockRemainingResponse], error) {
	sample := int(req.Msg.SampleDays)
	if sample <= 0 {
		sample = 30
	}
	from := time.Now().AddDate(0, 0, -sample)

	type row struct {
		MedicineID          string  `gorm:"column:medicine_id"`
		MedicineName        string  `gorm:"column:medicine_name"`
		SKU                 string  `gorm:"column:sku"`
		CurrentQty          int64   `gorm:"column:current_qty"`
		AvgDailyConsumption float64 `gorm:"column:avg_daily_consumption"`
		DaysRemaining       float64 `gorm:"column:days_remaining"`
	}
	var rows []row
	err := a.db.WithContext(ctx).Raw(`
		WITH window_sales AS (
			SELECT b.medicine_id, COALESCE(SUM(-sm.qty), 0) AS sold_qty
			FROM stock_movements sm
			JOIN batches b ON b.id = sm.batch_id
			WHERE sm.type = 'SALE' AND sm.created_at >= ?
			GROUP BY b.medicine_id
		),
		current_stock AS (
			SELECT b.medicine_id, COALESCE(SUM(sm.qty), 0) AS current_qty
			FROM batches b
			LEFT JOIN stock_movements sm ON sm.batch_id = b.id
			GROUP BY b.medicine_id
		)
		SELECT m.id AS medicine_id, m.name AS medicine_name, m.sku,
		       COALESCE(cs.current_qty, 0) AS current_qty,
		       COALESCE(ws.sold_qty, 0)::float / ? AS avg_daily_consumption,
		       CASE WHEN COALESCE(ws.sold_qty, 0) > 0
		            THEN COALESCE(cs.current_qty, 0)::float
		                 / (COALESCE(ws.sold_qty, 0)::float / ?)
		            ELSE NULL
		       END AS days_remaining
		FROM medicines m
		LEFT JOIN window_sales ws ON ws.medicine_id = m.id
		LEFT JOIN current_stock cs ON cs.medicine_id = m.id
		WHERE m.active = TRUE
		  AND COALESCE(cs.current_qty, 0) > 0
		ORDER BY days_remaining NULLS LAST, m.name
	`, from, sample, sample).Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*analyticsifacev1.DaysOfStockRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, &analyticsifacev1.DaysOfStockRow{
			MedicineId:          r.MedicineID,
			MedicineName:        r.MedicineName,
			Sku:                 r.SKU,
			CurrentQty:          r.CurrentQty,
			AvgDailyConsumption: r.AvgDailyConsumption,
			DaysRemaining:       r.DaysRemaining,
		})
	}
	return connect.NewResponse(&analyticsifacev1.GetDaysOfStockRemainingResponse{Rows: out}), nil
}

func (a *InventoryAnalytics) GetExpiryRiskForecast(
	ctx context.Context,
	_ *connect.Request[analyticsifacev1.GetExpiryRiskForecastRequest],
) (*connect.Response[analyticsifacev1.GetExpiryRiskForecastResponse], error) {
	now := time.Now()
	windows := []int{30, 90, 180}
	out := make([]*analyticsifacev1.ExpiryBucket, 0, len(windows))

	type row struct {
		QtyAtRisk   int64 `gorm:"column:qty_at_risk"`
		ValueAtRisk int64 `gorm:"column:value_at_risk"`
	}

	for _, win := range windows {
		var r row
		until := now.AddDate(0, 0, win)
		err := a.db.WithContext(ctx).Raw(`
			WITH batch_qty AS (
				SELECT b.id, b.cost_price, b.expiry_date,
				       COALESCE(SUM(sm.qty), 0) AS current_qty
				FROM batches b
				LEFT JOIN stock_movements sm ON sm.batch_id = b.id
				WHERE b.expiry_date >= ? AND b.expiry_date < ?
				GROUP BY b.id
			)
			SELECT COALESCE(SUM(current_qty), 0) AS qty_at_risk,
			       COALESCE(SUM(current_qty * cost_price), 0) AS value_at_risk
			FROM batch_qty
			WHERE current_qty > 0
		`, now, until).Scan(&r).Error
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out = append(out, &analyticsifacev1.ExpiryBucket{
			WindowDays:  int32(win),
			QtyAtRisk:   r.QtyAtRisk,
			ValueAtRisk: r.ValueAtRisk,
		})
	}

	return connect.NewResponse(&analyticsifacev1.GetExpiryRiskForecastResponse{Buckets: out}), nil
}
