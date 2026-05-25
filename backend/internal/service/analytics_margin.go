package service

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	analyticsifacev1 "github.com/apotech/backend/gen/analytics_iface/v1"
)

type MarginAnalytics struct {
	db *gorm.DB
}

func NewMarginAnalytics(db *gorm.DB) *MarginAnalytics { return &MarginAnalytics{db: db} }

// marginRows runs the shared "revenue + cogs + margin per medicine" query
// for [from, to] with the given order/limit. Reused by GetMarginPerMedicine
// and GetTopMargin.
func (a *MarginAnalytics) marginRows(
	ctx context.Context,
	from, to time.Time,
	orderBy string,
	limit int,
) ([]*analyticsifacev1.MarginRow, error) {
	type row struct {
		MedicineID   string  `gorm:"column:medicine_id"`
		MedicineName string  `gorm:"column:medicine_name"`
		SKU          string  `gorm:"column:sku"`
		Revenue      int64
		Cogs         int64
		GrossMargin  int64   `gorm:"column:gross_margin"`
		MarginPct    float64 `gorm:"column:margin_pct"`
		Qty          int64
	}
	var rows []row
	// Revenue comes from the sale line (line_total, in the selling unit). COGS
	// comes from the SALE stock_movements consumed by that line (base units ×
	// the consumed batch's cost_price) — correct under multi-unit + multi-batch.
	// `qty` is base units sold.
	err := a.db.WithContext(ctx).Raw(`
		SELECT si.medicine_id,
		       m.name AS medicine_name,
		       m.sku,
		       SUM(si.line_total) AS revenue,
		       SUM(COALESCE(c.cogs, 0)) AS cogs,
		       SUM(si.line_total) - SUM(COALESCE(c.cogs, 0)) AS gross_margin,
		       CASE WHEN SUM(si.line_total) > 0
		            THEN (SUM(si.line_total) - SUM(COALESCE(c.cogs, 0)))::float
		                 / SUM(si.line_total)
		            ELSE 0
		       END AS margin_pct,
		       SUM(si.base_qty) AS qty
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		JOIN medicines m ON m.id = si.medicine_id
		LEFT JOIN (
		  SELECT sm.sale_item_id, SUM(ABS(sm.qty) * COALESCE(b.cost_price, 0)) AS cogs
		  FROM stock_movements sm
		  JOIN batches b ON b.id = sm.batch_id
		  WHERE sm.type = 'SALE' AND sm.sale_item_id IS NOT NULL
		  GROUP BY sm.sale_item_id
		) c ON c.sale_item_id = si.id
		WHERE s.status = 'COMPLETED'
		  AND s.completed_at >= ? AND s.completed_at < ?
		GROUP BY si.medicine_id, m.name, m.sku
		ORDER BY `+orderBy+`
		LIMIT ?
	`, from, to, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]*analyticsifacev1.MarginRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, &analyticsifacev1.MarginRow{
			MedicineId:   r.MedicineID,
			MedicineName: r.MedicineName,
			Sku:          r.SKU,
			Revenue:      r.Revenue,
			Cogs:         r.Cogs,
			GrossMargin:  r.GrossMargin,
			MarginPct:    r.MarginPct,
			Qty:          r.Qty,
		})
	}
	return out, nil
}

func (a *MarginAnalytics) GetMarginPerMedicine(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetMarginPerMedicineRequest],
) (*connect.Response[analyticsifacev1.GetMarginPerMedicineResponse], error) {
	from, to := dateRangeOrDefault(req.Msg.FromUnix, req.Msg.ToUnix)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := a.marginRows(ctx, from, to, "medicine_name", limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&analyticsifacev1.GetMarginPerMedicineResponse{Rows: rows}), nil
}

func (a *MarginAnalytics) GetTopMargin(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetTopMarginRequest],
) (*connect.Response[analyticsifacev1.GetTopMarginResponse], error) {
	from, to := dateRangeOrDefault(req.Msg.FromUnix, req.Msg.ToUnix)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := a.marginRows(ctx, from, to, "gross_margin DESC", limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&analyticsifacev1.GetTopMarginResponse{Rows: rows}), nil
}

func (a *MarginAnalytics) GetSupplierCostTrend(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetSupplierCostTrendRequest],
) (*connect.Response[analyticsifacev1.GetSupplierCostTrendResponse], error) {
	if req.Msg.SupplierId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("supplier_id required"))
	}

	type row struct {
		ReceivedAt   string `gorm:"column:received_at"`
		CostPrice    int64  `gorm:"column:cost_price"`
		BatchNumber  string `gorm:"column:batch_number"`
		MedicineName string `gorm:"column:medicine_name"`
	}
	var rows []row
	q := a.db.WithContext(ctx).
		Table("batches b").
		Joins("JOIN medicines m ON m.id = b.medicine_id").
		Where("b.supplier_id = ?", req.Msg.SupplierId)
	if req.Msg.MedicineId != "" {
		q = q.Where("b.medicine_id = ?", req.Msg.MedicineId)
	}
	err := q.
		Select(`TO_CHAR(b.received_at, 'YYYY-MM-DD') AS received_at,
		        b.cost_price,
		        b.batch_number,
		        m.name AS medicine_name`).
		Order("b.received_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*analyticsifacev1.CostTrendPoint, 0, len(rows))
	for _, r := range rows {
		out = append(out, &analyticsifacev1.CostTrendPoint{
			ReceivedAt:   r.ReceivedAt,
			CostPrice:    r.CostPrice,
			BatchNumber:  r.BatchNumber,
			MedicineName: r.MedicineName,
		})
	}
	return connect.NewResponse(&analyticsifacev1.GetSupplierCostTrendResponse{Points: out}), nil
}
