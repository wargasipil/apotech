package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
)

type Medicines struct {
	db *gorm.DB
}

func NewMedicines(db *gorm.DB) *Medicines { return &Medicines{db: db} }

func (m *Medicines) ListMedicines(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListMedicinesRequest],
) (*connect.Response[inventoryifacev1.ListMedicinesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	query := strings.TrimSpace(req.Msg.Query)

	applyFilters := func(q *gorm.DB) *gorm.DB {
		if !req.Msg.IncludeInactive {
			q = q.Where("active = ?", true)
		}
		if query != "" {
			pattern := "%" + query + "%"
			q = q.Where("name ILIKE ? OR sku ILIKE ?", pattern, pattern)
		}
		return q
	}

	var total int64
	if err := applyFilters(m.db.WithContext(ctx).Model(&model.Medicine{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Medicine
	if err := applyFilters(m.db.WithContext(ctx).Model(&model.Medicine{})).
		Order("name").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Medicine, 0, len(rows))
	for i := range rows {
		out = append(out, medicineToProto(&rows[i]))
	}
	if err := m.enrichStock(ctx, caller, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.ListMedicinesResponse{
		Medicines: out,
		Total:     int32(total),
	}), nil
}

// enrichStock fills ready_stock (on-hand in the active warehouse) + on_order_stock
// (incoming on open POs) for a page of medicines. Two batched grouped queries.
func (m *Medicines) enrichStock(
	ctx context.Context,
	caller auth.Principal,
	meds []*inventoryifacev1.Medicine,
) error {
	if len(meds) == 0 {
		return nil
	}
	ids := make([]string, 0, len(meds))
	for _, md := range meds {
		ids = append(ids, md.Id)
	}
	warehouseID, err := resolveWarehouse(ctx, m.db, caller)
	if err != nil {
		return err
	}

	// Ready: SUM(stock_movements.qty) per medicine in the active warehouse.
	type readyRow struct {
		MedicineID string `gorm:"column:medicine_id"`
		Qty        int64  `gorm:"column:qty"`
	}
	var readyRows []readyRow
	if err := m.db.WithContext(ctx).
		Table("batches AS b").
		Select("b.medicine_id AS medicine_id, COALESCE(SUM(sm.qty), 0) AS qty").
		Joins("LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", warehouseID).
		Where("b.medicine_id IN ?", ids).
		Group("b.medicine_id").Scan(&readyRows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	ready := make(map[string]int64, len(readyRows))
	for _, r := range readyRows {
		ready[r.MedicineID] = r.Qty
	}

	// On-order: SUM(ordered_qty - received_qty) per medicine on open POs.
	type orderRow struct {
		MedicineID string `gorm:"column:medicine_id"`
		Qty        int64  `gorm:"column:qty"`
	}
	var orderRows []orderRow
	if err := m.db.WithContext(ctx).
		Table("purchase_order_items AS poi").
		Select("poi.medicine_id AS medicine_id, COALESCE(SUM(poi.ordered_qty - poi.received_qty), 0) AS qty").
		Joins("JOIN purchase_orders po ON po.id = poi.purchase_order_id").
		Where("poi.medicine_id IN ? AND po.status NOT IN ?", ids,
			[]string{poStatusVoided, poStatusClosed, poStatusReceived}).
		Group("poi.medicine_id").Scan(&orderRows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	onOrder := make(map[string]int64, len(orderRows))
	for _, r := range orderRows {
		onOrder[r.MedicineID] = r.Qty
	}

	for _, md := range meds {
		md.ReadyStock = ready[md.Id]
		md.OnOrderStock = onOrder[md.Id]
	}
	return nil
}

func (m *Medicines) GetMedicine(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetMedicineRequest],
) (*connect.Response[inventoryifacev1.GetMedicineResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	med, err := m.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	out := medicineToProto(med)
	// Fill ready_stock (active warehouse) + on_order_stock so the detail page
	// shows the same figures as the list.
	if err := m.enrichStock(ctx, caller, []*inventoryifacev1.Medicine{out}); err != nil {
		return nil, err
	}
	// Last restock = the most recent batch received for this medicine, with that
	// batch's supplier. Detail-only (kept out of the list's enrichStock).
	var rr struct {
		ReceivedAt   time.Time `gorm:"column:received_at"`
		SupplierName string    `gorm:"column:supplier_name"`
	}
	if err := m.db.WithContext(ctx).
		Table("batches b").
		Select("b.received_at AS received_at, COALESCE(s.name, '') AS supplier_name").
		Joins("LEFT JOIN suppliers s ON s.id = b.supplier_id").
		Where("b.medicine_id = ?", med.ID).
		Order("b.received_at DESC, b.created_at DESC").
		Limit(1).Scan(&rr).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !rr.ReceivedAt.IsZero() {
		out.LastRestockDate = rr.ReceivedAt.Format(dateLayout)
		out.LastRestockSupplier = rr.SupplierName
	}
	// Total stock (global on-hand across all warehouses) + valuation at cost.
	// Σ(qty × cost) over movements == Σ_batch(qty)×cost (cost is per-batch).
	var v struct {
		TotalStock int64 `gorm:"column:total_stock"`
		Valuation  int64 `gorm:"column:valuation"`
	}
	if err := m.db.WithContext(ctx).
		Table("stock_movements sm").
		Joins("JOIN batches b ON b.id = sm.batch_id").
		Where("b.medicine_id = ?", med.ID).
		Select("COALESCE(SUM(sm.qty), 0) AS total_stock, COALESCE(SUM(sm.qty * b.cost_price), 0) AS valuation").
		Scan(&v).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out.TotalStock = v.TotalStock
	out.StockValuation = v.Valuation
	return connect.NewResponse(&inventoryifacev1.GetMedicineResponse{Medicine: out}), nil
}

func (m *Medicines) CreateMedicine(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateMedicineRequest],
) (*connect.Response[inventoryifacev1.CreateMedicineResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	sku := strings.TrimSpace(req.Msg.Sku)
	name := strings.TrimSpace(req.Msg.Name)
	unit := strings.TrimSpace(req.Msg.Unit)
	if sku == "" || name == "" || unit == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sku, name, unit required"))
	}
	if req.Msg.UnitPrice < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unit_price must be >= 0"))
	}

	med := model.Medicine{
		SKU:                  sku,
		Name:                 name,
		Manufacturer:         strings.TrimSpace(req.Msg.Manufacturer),
		Unit:                 unit,
		UnitPrice:            req.Msg.UnitPrice,
		PrescriptionRequired: req.Msg.PrescriptionRequired,
		Active:               true,
	}

	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&med).Error; err != nil {
			return fmt.Errorf("create medicine: %w", err)
		}
		price := model.MedicinePrice{
			MedicineID:    med.ID,
			UnitPrice:     med.UnitPrice,
			EffectiveFrom: time.Now(),
			ChangedBy:     caller.UserID,
		}
		if err := tx.Create(&price).Error; err != nil {
			return fmt.Errorf("create initial price: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, err)
	}
	return connect.NewResponse(&inventoryifacev1.CreateMedicineResponse{Medicine: medicineToProto(&med)}), nil
}

func (m *Medicines) UpdateMedicine(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateMedicineRequest],
) (*connect.Response[inventoryifacev1.UpdateMedicineResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	med, err := m.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Msg.Name)
	unit := strings.TrimSpace(req.Msg.Unit)
	if name == "" || unit == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and unit required"))
	}
	if req.Msg.UnitPrice < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unit_price must be >= 0"))
	}

	priceChanged := req.Msg.UnitPrice != med.UnitPrice

	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"name":                  name,
			"manufacturer":          strings.TrimSpace(req.Msg.Manufacturer),
			"unit":                  unit,
			"prescription_required": req.Msg.PrescriptionRequired,
		}
		if priceChanged {
			updates["unit_price"] = req.Msg.UnitPrice
		}
		if err := tx.Model(med).Updates(updates).Error; err != nil {
			return err
		}

		if !priceChanged {
			return nil
		}
		now := time.Now()
		// Close the current open price row.
		if err := tx.Model(&model.MedicinePrice{}).
			Where("medicine_id = ? AND effective_to IS NULL", med.ID).
			Update("effective_to", now).Error; err != nil {
			return fmt.Errorf("close current price: %w", err)
		}
		// Insert the new open row.
		newPrice := model.MedicinePrice{
			MedicineID:    med.ID,
			UnitPrice:     req.Msg.UnitPrice,
			EffectiveFrom: now,
			ChangedBy:     caller.UserID,
		}
		if err := tx.Create(&newPrice).Error; err != nil {
			return fmt.Errorf("insert new price: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	// Refresh from DB so response reflects the new state.
	med, err = m.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.UpdateMedicineResponse{Medicine: medicineToProto(med)}), nil
}

func (m *Medicines) ArchiveMedicine(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ArchiveMedicineRequest],
) (*connect.Response[inventoryifacev1.ArchiveMedicineResponse], error) {
	med, err := m.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := m.db.WithContext(ctx).Model(med).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	med.Active = false
	return connect.NewResponse(&inventoryifacev1.ArchiveMedicineResponse{Medicine: medicineToProto(med)}), nil
}

func (m *Medicines) SearchMedicines(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SearchMedicinesRequest],
) (*connect.Response[inventoryifacev1.SearchMedicinesResponse], error) {
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := m.db.WithContext(ctx).Order("name").Limit(limit)
	if !req.Msg.IncludeInactive {
		q = q.Where("active = ?", true)
	}
	if query != "" {
		pattern := "%" + query + "%"
		q = q.Where("name ILIKE ? OR sku ILIKE ? OR manufacturer ILIKE ?", pattern, pattern, pattern)
	}
	var rows []model.Medicine
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Medicine, 0, len(rows))
	for _, r := range rows {
		out = append(out, medicineToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.SearchMedicinesResponse{Medicines: out}), nil
}

func (m *Medicines) ListMedicinePrices(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListMedicinePricesRequest],
) (*connect.Response[inventoryifacev1.ListMedicinePricesResponse], error) {
	if req.Msg.MedicineId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("medicine_id required"))
	}
	var rows []model.MedicinePrice
	err := m.db.WithContext(ctx).
		Where("medicine_id = ?", req.Msg.MedicineId).
		Order("effective_from DESC").
		Find(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.MedicinePrice, 0, len(rows))
	for _, r := range rows {
		out = append(out, medicinePriceToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.ListMedicinePricesResponse{Prices: out}), nil
}

func (m *Medicines) load(ctx context.Context, id string) (*model.Medicine, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var med model.Medicine
	err := m.db.WithContext(ctx).Where("id = ?", id).First(&med).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("medicine %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &med, nil
}

func medicineToProto(m *model.Medicine) *inventoryifacev1.Medicine {
	return &inventoryifacev1.Medicine{
		Id:                   m.ID,
		Sku:                  m.SKU,
		Name:                 m.Name,
		Manufacturer:         m.Manufacturer,
		Unit:                 m.Unit,
		UnitPrice:            m.UnitPrice,
		PrescriptionRequired: m.PrescriptionRequired,
		Active:               m.Active,
		CreatedAt:            m.CreatedAt.Unix(),
	}
}

func medicinePriceToProto(p *model.MedicinePrice) *inventoryifacev1.MedicinePrice {
	out := &inventoryifacev1.MedicinePrice{
		Id:            p.ID,
		MedicineId:    p.MedicineID,
		UnitPrice:     p.UnitPrice,
		EffectiveFrom: p.EffectiveFrom.Unix(),
		ChangedBy:     p.ChangedBy,
	}
	if p.EffectiveTo != nil {
		out.EffectiveTo = p.EffectiveTo.Unix()
	}
	return out
}
