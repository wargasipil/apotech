package service

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	analyticsifacev1 "github.com/apotech/backend/gen/analytics_iface/v1"
)

type SalesAnalytics struct {
	db *gorm.DB
}

func NewSalesAnalytics(db *gorm.DB) *SalesAnalytics { return &SalesAnalytics{db: db} }

// dateRangeOrDefault returns a [from, to] time pair; defaults to last 30 days
// if both are zero.
func dateRangeOrDefault(fromUnix, toUnix int64) (time.Time, time.Time) {
	now := time.Now()
	var from, to time.Time
	if toUnix > 0 {
		to = time.Unix(toUnix, 0)
	} else {
		to = now
	}
	if fromUnix > 0 {
		from = time.Unix(fromUnix, 0)
	} else {
		from = to.AddDate(0, 0, -30)
	}
	return from, to
}

func (s *SalesAnalytics) GetRevenueTrend(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetRevenueTrendRequest],
) (*connect.Response[analyticsifacev1.GetRevenueTrendResponse], error) {
	from, to := dateRangeOrDefault(req.Msg.FromUnix, req.Msg.ToUnix)

	var truncFmt string
	switch req.Msg.Granularity {
	case analyticsifacev1.Granularity_GRANULARITY_MONTH:
		truncFmt = "month"
	case analyticsifacev1.Granularity_GRANULARITY_WEEK:
		truncFmt = "week"
	default:
		truncFmt = "day"
	}

	type row struct {
		Bucket    time.Time `gorm:"column:bucket"`
		Revenue   int64
		SaleCount int64 `gorm:"column:sale_count"`
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Table("sales").
		Select("DATE_TRUNC(?, completed_at) AS bucket, COALESCE(SUM(total), 0) AS revenue, COUNT(*) AS sale_count", truncFmt).
		Where("status = ? AND completed_at >= ? AND completed_at < ?", saleStatusCompleted, from, to).
		Group("bucket").
		Order("bucket").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*analyticsifacev1.RevenueTrendPoint, 0, len(rows))
	for _, r := range rows {
		out = append(out, &analyticsifacev1.RevenueTrendPoint{
			Bucket:    bucketLabel(r.Bucket, truncFmt),
			Revenue:   r.Revenue,
			SaleCount: r.SaleCount,
		})
	}
	return connect.NewResponse(&analyticsifacev1.GetRevenueTrendResponse{Points: out}), nil
}

func bucketLabel(t time.Time, granularity string) string {
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

func (s *SalesAnalytics) GetTopSellers(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetTopSellersRequest],
) (*connect.Response[analyticsifacev1.GetTopSellersResponse], error) {
	from, to := dateRangeOrDefault(req.Msg.FromUnix, req.Msg.ToUnix)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	orderBy := "qty DESC"
	if req.Msg.Metric == analyticsifacev1.SortMetric_SORT_METRIC_REVENUE {
		orderBy = "revenue DESC"
	}

	type row struct {
		MedicineID   string `gorm:"column:medicine_id"`
		MedicineName string `gorm:"column:medicine_name"`
		SKU          string `gorm:"column:sku"`
		Qty          int64
		Revenue      int64
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Table("sale_items si").
		Joins("JOIN sales s ON s.id = si.sale_id").
		Joins("JOIN medicines m ON m.id = si.medicine_id").
		Where("s.status = ? AND s.completed_at >= ? AND s.completed_at < ?",
			saleStatusCompleted, from, to).
		Select("si.medicine_id, m.name AS medicine_name, m.sku, SUM(si.base_qty) AS qty, SUM(si.line_total) AS revenue").
		Group("si.medicine_id, m.name, m.sku").
		Order(orderBy).
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*analyticsifacev1.TopSeller, 0, len(rows))
	for _, r := range rows {
		out = append(out, &analyticsifacev1.TopSeller{
			MedicineId:   r.MedicineID,
			MedicineName: r.MedicineName,
			Sku:          r.SKU,
			Qty:          r.Qty,
			Revenue:      r.Revenue,
		})
	}
	return connect.NewResponse(&analyticsifacev1.GetTopSellersResponse{Items: out}), nil
}

func (s *SalesAnalytics) GetPaymentMix(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetPaymentMixRequest],
) (*connect.Response[analyticsifacev1.GetPaymentMixResponse], error) {
	from, to := dateRangeOrDefault(req.Msg.FromUnix, req.Msg.ToUnix)

	type row struct {
		PaymentSource string `gorm:"column:payment_source"`
		Revenue       int64
		SaleCount     int64 `gorm:"column:sale_count"`
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Table("sales").
		Where("status = ? AND completed_at >= ? AND completed_at < ?",
			saleStatusCompleted, from, to).
		Select("COALESCE(payment_source, '') AS payment_source, COALESCE(SUM(total), 0) AS revenue, COUNT(*) AS sale_count").
		Group("payment_source").
		Order("revenue DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*analyticsifacev1.PaymentMixSlice, 0, len(rows))
	for _, r := range rows {
		out = append(out, &analyticsifacev1.PaymentMixSlice{
			PaymentSource: r.PaymentSource,
			Revenue:       r.Revenue,
			SaleCount:     r.SaleCount,
		})
	}
	return connect.NewResponse(&analyticsifacev1.GetPaymentMixResponse{Slices: out}), nil
}

func (s *SalesAnalytics) GetSalesByCashier(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetSalesByCashierRequest],
) (*connect.Response[analyticsifacev1.GetSalesByCashierResponse], error) {
	from, to := dateRangeOrDefault(req.Msg.FromUnix, req.Msg.ToUnix)

	type row struct {
		UserID    string `gorm:"column:user_id"`
		UserEmail string `gorm:"column:user_email"`
		UserName  string `gorm:"column:user_name"`
		Revenue   int64
		SaleCount int64 `gorm:"column:sale_count"`
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Table("sales s").
		Joins("JOIN users u ON u.id = s.cashier_user_id").
		Where("s.status = ? AND s.completed_at >= ? AND s.completed_at < ?",
			saleStatusCompleted, from, to).
		Select("u.id AS user_id, u.email AS user_email, u.name AS user_name, COALESCE(SUM(s.total), 0) AS revenue, COUNT(*) AS sale_count").
		Group("u.id, u.email, u.name").
		Order("revenue DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*analyticsifacev1.CashierTotals, 0, len(rows))
	for _, r := range rows {
		out = append(out, &analyticsifacev1.CashierTotals{
			UserId:    r.UserID,
			UserEmail: r.UserEmail,
			UserName:  r.UserName,
			Revenue:   r.Revenue,
			SaleCount: r.SaleCount,
		})
	}
	return connect.NewResponse(&analyticsifacev1.GetSalesByCashierResponse{Items: out}), nil
}

func (s *SalesAnalytics) GetHourOfDayHeatmap(
	ctx context.Context,
	req *connect.Request[analyticsifacev1.GetHourOfDayHeatmapRequest],
) (*connect.Response[analyticsifacev1.GetHourOfDayHeatmapResponse], error) {
	from, to := dateRangeOrDefault(req.Msg.FromUnix, req.Msg.ToUnix)

	type row struct {
		DayOfWeek int32 `gorm:"column:day_of_week"`
		Hour      int32
		SaleCount int64 `gorm:"column:sale_count"`
		Revenue   int64
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Table("sales").
		Where("status = ? AND completed_at >= ? AND completed_at < ?",
			saleStatusCompleted, from, to).
		Select(`EXTRACT(DOW FROM completed_at)::int AS day_of_week,
		        EXTRACT(HOUR FROM completed_at)::int AS hour,
		        COUNT(*) AS sale_count,
		        COALESCE(SUM(total), 0) AS revenue`).
		Group("day_of_week, hour").
		Order("day_of_week, hour").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*analyticsifacev1.HeatmapCell, 0, len(rows))
	for _, r := range rows {
		out = append(out, &analyticsifacev1.HeatmapCell{
			DayOfWeek: r.DayOfWeek,
			Hour:      r.Hour,
			SaleCount: r.SaleCount,
			Revenue:   r.Revenue,
		})
	}
	return connect.NewResponse(&analyticsifacev1.GetHourOfDayHeatmapResponse{Cells: out}), nil
}
