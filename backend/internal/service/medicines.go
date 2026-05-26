package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	if err := m.attachUnits(ctx, out); err != nil {
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
	warehouseID, err := resolveWarehouse(ctx, m.db, caller)
	if err != nil {
		return nil, err
	}
	out := medicineToProto(med)
	// Fill ready_stock (active warehouse) + on_order_stock so the detail page
	// shows the same figures as the list.
	if err := m.enrichStock(ctx, caller, []*inventoryifacev1.Medicine{out}); err != nil {
		return nil, err
	}
	// Last restock = the most recent stock arrival INTO THE ACTIVE WAREHOUSE for
	// this medicine (any positive movement: purchase, transfer-in, +adjustment),
	// with that batch's supplier. Detail-only (kept out of the list's enrichStock).
	var rr struct {
		ReceivedAt   time.Time `gorm:"column:received_at"`
		SupplierName string    `gorm:"column:supplier_name"`
	}
	if err := m.db.WithContext(ctx).
		Table("stock_movements sm").
		Select("b.received_at AS received_at, COALESCE(s.name, '') AS supplier_name").
		Joins("JOIN batches b ON b.id = sm.batch_id").
		Joins("LEFT JOIN suppliers s ON s.id = b.supplier_id").
		Where("b.medicine_id = ? AND sm.warehouse_id = ? AND sm.qty > 0", med.ID, warehouseID).
		Order("sm.created_at DESC").
		Limit(1).Scan(&rr).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !rr.ReceivedAt.IsZero() {
		out.LastRestockDate = rr.ReceivedAt.Format(dateLayout)
		out.LastRestockSupplier = rr.SupplierName
	}
	// Total stock (on-hand in the active warehouse) + valuation at cost.
	// Σ(qty × cost) over movements == Σ_batch(qty)×cost (cost is per-batch).
	// Scoped to the active warehouse, so this equals ready_stock — the detail
	// page renders only valuation from it (the redundant "Total stock" tile is
	// gone); total_stock stays on the proto, just unrendered.
	var v struct {
		TotalStock int64 `gorm:"column:total_stock"`
		Valuation  int64 `gorm:"column:valuation"`
	}
	if err := m.db.WithContext(ctx).
		Table("stock_movements sm").
		Joins("JOIN batches b ON b.id = sm.batch_id").
		Where("b.medicine_id = ? AND sm.warehouse_id = ?", med.ID, warehouseID).
		Select("COALESCE(SUM(sm.qty), 0) AS total_stock, COALESCE(SUM(sm.qty * b.cost_price), 0) AS valuation").
		Scan(&v).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out.TotalStock = v.TotalStock
	out.StockValuation = v.Valuation
	// Reference cost = the latest purchase cost (most recent batch's cost_price,
	// per base unit, global). Drives the markup/margin readout in the medicine
	// form. Detail-only; 0 when the medicine has no batch yet.
	var refCost *int64
	if err := m.db.WithContext(ctx).
		Model(&model.Batch{}).
		Where("medicine_id = ?", med.ID).
		Order("received_at DESC, created_at DESC").
		Limit(1).
		Select("cost_price").
		Scan(&refCost).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if refCost != nil {
		out.ReferenceCost = *refCost
	}
	if err := m.attachUnits(ctx, []*inventoryifacev1.Medicine{out}); err != nil {
		return nil, err
	}
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
		// Base unit (factor 1) + any additional units supplied.
		if err := syncMedicineUnits(tx, &med, req.Msg.Units, caller.UserID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		var ce *connect.Error
		if errors.As(err, &ce) {
			return nil, err // unit validation error — keep its code
		}
		return nil, connect.NewError(connect.CodeAlreadyExists, err) // likely dup SKU
	}
	out := medicineToProto(&med)
	if err := m.attachUnits(ctx, []*inventoryifacev1.Medicine{out}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.CreateMedicineResponse{Medicine: out}), nil
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
		// Lock the medicine row so concurrent price edits serialize — otherwise the
		// close-open-row + insert-new-open-row price-version sequence (here and in
		// syncMedicineUnits/recordUnitPrice) can collide on the *_open_idx partial
		// unique index and fail spuriously.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", med.ID).First(&model.Medicine{}).Error; err != nil {
			return err
		}
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

		if priceChanged {
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
		}

		// Sync units against the new base name/price.
		med.Unit = unit
		med.UnitPrice = req.Msg.UnitPrice
		if err := syncMedicineUnits(tx, med, req.Msg.Units, caller.UserID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		var ce *connect.Error
		if errors.As(err, &ce) {
			return nil, err
		}
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	// Refresh from DB so response reflects the new state.
	med, err = m.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	out := medicineToProto(med)
	if err := m.attachUnits(ctx, []*inventoryifacev1.Medicine{out}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.UpdateMedicineResponse{Medicine: out}), nil
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
	for i := range rows {
		out = append(out, medicineToProto(&rows[i]))
	}
	if err := m.attachUnits(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.SearchMedicinesResponse{Medicines: out}), nil
}

// ResolveMedicines returns minimal display refs for a set of ids. Unknown ids
// are omitted; empty input returns an empty list. No enrich, no preload.
func (m *Medicines) ResolveMedicines(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ResolveMedicinesRequest],
) (*connect.Response[inventoryifacev1.ResolveMedicinesResponse], error) {
	ids := dedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&inventoryifacev1.ResolveMedicinesResponse{}), nil
	}
	type row struct {
		ID   string `gorm:"column:id"`
		Name string `gorm:"column:name"`
		SKU  string `gorm:"column:sku"`
	}
	var rows []row
	if err := m.db.WithContext(ctx).
		Model(&model.Medicine{}).
		Select("id, name, sku").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.MedicineRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.MedicineRef{Id: r.ID, Name: r.Name, Sku: r.SKU})
	}
	return connect.NewResponse(&inventoryifacev1.ResolveMedicinesResponse{Medicines: out}), nil
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

// ListMedicineUnitPrices returns the per-unit sell-price history for a medicine,
// joined to the unit name and ordered base-first then newest-first.
func (m *Medicines) ListMedicineUnitPrices(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListMedicineUnitPricesRequest],
) (*connect.Response[inventoryifacev1.ListMedicineUnitPricesResponse], error) {
	if req.Msg.MedicineId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("medicine_id required"))
	}
	type row struct {
		ID             string     `gorm:"column:id"`
		MedicineUnitID string     `gorm:"column:medicine_unit_id"`
		UnitName       string     `gorm:"column:unit_name"`
		UnitSellPrice  int64      `gorm:"column:unit_sell_price"`
		EffectiveFrom  time.Time  `gorm:"column:effective_from"`
		EffectiveTo    *time.Time `gorm:"column:effective_to"`
		ChangedBy      *string    `gorm:"column:changed_by"`
	}
	var rows []row
	err := m.db.WithContext(ctx).
		Table("medicine_unit_prices mup").
		Select(`mup.id, mup.medicine_unit_id, mu.name AS unit_name, mup.unit_sell_price,
		        mup.effective_from, mup.effective_to, mup.changed_by`).
		Joins("JOIN medicine_units mu ON mu.id = mup.medicine_unit_id").
		Where("mu.medicine_id = ?", req.Msg.MedicineId).
		Order("mu.is_base DESC, mu.factor ASC, mup.effective_from DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.MedicineUnitPrice, 0, len(rows))
	for _, r := range rows {
		p := &inventoryifacev1.MedicineUnitPrice{
			Id:             r.ID,
			MedicineUnitId: r.MedicineUnitID,
			UnitName:       r.UnitName,
			UnitSellPrice:  r.UnitSellPrice,
			EffectiveFrom:  r.EffectiveFrom.Unix(),
		}
		if r.EffectiveTo != nil {
			p.EffectiveTo = r.EffectiveTo.Unix()
		}
		if r.ChangedBy != nil {
			p.ChangedBy = *r.ChangedBy
		}
		out = append(out, p)
	}
	return connect.NewResponse(&inventoryifacev1.ListMedicineUnitPricesResponse{Prices: out}), nil
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

func medicineUnitToProto(u *model.MedicineUnit) *inventoryifacev1.MedicineUnit {
	return &inventoryifacev1.MedicineUnit{
		Id:          u.ID,
		MedicineId:  u.MedicineID,
		Name:        u.Name,
		Factor:      u.Factor,
		IsBase:      u.IsBase,
		SellPrice:   u.SellPrice,
		Sellable:    u.Sellable,
		Purchasable: u.Purchasable,
		SortOrder:   int32(u.SortOrder),
		Active:      u.Active,
	}
}

// attachUnits batch-loads each medicine's active units (base first, then by
// factor) and sets them on the protos. No N+1.
func (m *Medicines) attachUnits(ctx context.Context, meds []*inventoryifacev1.Medicine) error {
	if len(meds) == 0 {
		return nil
	}
	ids := make([]string, 0, len(meds))
	for _, md := range meds {
		ids = append(ids, md.Id)
	}
	var rows []model.MedicineUnit
	if err := m.db.WithContext(ctx).
		Where("medicine_id IN ? AND active", ids).
		Order("is_base DESC, factor ASC").
		Find(&rows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	byMed := make(map[string][]*inventoryifacev1.MedicineUnit, len(meds))
	for i := range rows {
		byMed[rows[i].MedicineID] = append(byMed[rows[i].MedicineID], medicineUnitToProto(&rows[i]))
	}
	for _, md := range meds {
		md.Units = byMed[md.Id]
	}
	return nil
}

// syncMedicineUnits upserts a medicine's units inside a tx: the base unit is
// derived from med.Unit/med.UnitPrice (factor 1); `inputs` are the larger
// (non-base) units. Non-base units absent from `inputs` are deactivated.
func syncMedicineUnits(tx *gorm.DB, med *model.Medicine, inputs []*inventoryifacev1.MedicineUnitInput, changedBy string) error {
	// Upsert the base unit.
	var base model.MedicineUnit
	err := tx.Where("medicine_id = ? AND is_base", med.ID).First(&base).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		base = model.MedicineUnit{
			MedicineID: med.ID, Name: med.Unit, Factor: 1, IsBase: true,
			SellPrice: med.UnitPrice, Sellable: true, Purchasable: true, Active: true,
		}
		if err := tx.Create(&base).Error; err != nil {
			return err
		}
		if err := recordUnitPrice(tx, base.ID, med.UnitPrice, changedBy); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if err := tx.Model(&model.MedicineUnit{}).Where("id = ?", base.ID).
			Updates(map[string]any{"name": med.Unit, "sell_price": med.UnitPrice, "active": true}).Error; err != nil {
			return err
		}
		if base.SellPrice != med.UnitPrice {
			if err := recordUnitPrice(tx, base.ID, med.UnitPrice, changedBy); err != nil {
				return err
			}
		}
	}

	keptIDs := []string{base.ID}
	for _, in := range inputs {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("unit name required"))
		}
		if in.Factor <= 1 {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unit %q factor must be > 1", name))
		}
		if in.SellPrice < 0 {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("sell_price must be >= 0"))
		}
		if in.Id != "" {
			var existing model.MedicineUnit
			if err := tx.Where("id = ? AND medicine_id = ?", in.Id, med.ID).First(&existing).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.MedicineUnit{}).
				Where("id = ? AND medicine_id = ?", in.Id, med.ID).
				Updates(map[string]any{
					"name": name, "factor": in.Factor, "sell_price": in.SellPrice,
					"sellable": in.Sellable, "purchasable": in.Purchasable,
					"sort_order": int(in.SortOrder), "active": true,
				}).Error; err != nil {
				return err
			}
			if existing.SellPrice != in.SellPrice {
				if err := recordUnitPrice(tx, in.Id, in.SellPrice, changedBy); err != nil {
					return err
				}
			}
			keptIDs = append(keptIDs, in.Id)
		} else {
			row := model.MedicineUnit{
				MedicineID: med.ID, Name: name, Factor: in.Factor, IsBase: false,
				SellPrice: in.SellPrice, Sellable: in.Sellable, Purchasable: in.Purchasable,
				SortOrder: int(in.SortOrder), Active: true,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			if err := recordUnitPrice(tx, row.ID, in.SellPrice, changedBy); err != nil {
				return err
			}
			keptIDs = append(keptIDs, row.ID)
		}
	}
	// Deactivate non-base units that were removed from the set.
	return tx.Model(&model.MedicineUnit{}).
		Where("medicine_id = ? AND is_base = false AND id NOT IN ?", med.ID, keptIDs).
		Update("active", false).Error
}

// recordUnitPrice closes a unit's open price row (if any) and inserts a new open
// row, mirroring the medicine_prices versioning for the base price.
func recordUnitPrice(tx *gorm.DB, unitID string, newPrice int64, changedBy string) error {
	now := time.Now()
	if err := tx.Model(&model.MedicineUnitPrice{}).
		Where("medicine_unit_id = ? AND effective_to IS NULL", unitID).
		Update("effective_to", now).Error; err != nil {
		return err
	}
	row := model.MedicineUnitPrice{
		MedicineUnitID: unitID,
		UnitSellPrice:  newPrice,
		EffectiveFrom:  now,
	}
	if changedBy != "" {
		row.ChangedBy = &changedBy
	}
	return tx.Create(&row).Error
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
