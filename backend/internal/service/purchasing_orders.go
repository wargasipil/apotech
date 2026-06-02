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

	purchasingifacev1 "github.com/apotech/backend/gen/purchasing_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
)

const (
	poStatusDraft              = "DRAFT"
	poStatusSent               = "SENT"
	poStatusPartiallyReceived  = "PARTIALLY_RECEIVED"
	poStatusReceived           = "RECEIVED"
	poStatusClosed             = "CLOSED"
	poStatusVoided             = "VOIDED"
	defaultPPNRate             = 11 // Indonesia's standard PPN rate as of 2026
)

// computePOTotals computes PPN-exclusive purchase totals.
// dpp = subtotal − cart_discount; ppn = ppnEnabled ? round(dpp × rate%) : 0;
// ordered_total = dpp + ppn. cartDiscount is clamped to [0, subtotal]. rate
// falls back to the Indonesia default (11) when 0/negative.
func computePOTotals(items []model.PurchaseOrderItem, cartDiscount int64, ppnEnabled bool, ppnRate int32) (subtotal, discount int64, effectiveRate int32, ppn, total int64) {
	for _, it := range items {
		subtotal += it.Subtotal
	}
	if cartDiscount < 0 {
		cartDiscount = 0
	}
	if cartDiscount > subtotal {
		cartDiscount = subtotal
	}
	dpp := subtotal - cartDiscount
	effectiveRate = ppnRate
	if effectiveRate <= 0 {
		effectiveRate = defaultPPNRate
	}
	if effectiveRate > 100 {
		effectiveRate = 100
	}
	if ppnEnabled {
		ppn = (dpp*int64(effectiveRate) + 50) / 100 // round to nearest rupiah
	}
	return subtotal, cartDiscount, effectiveRate, ppn, dpp + ppn
}

type PurchaseOrders struct {
	db *gorm.DB
}

func NewPurchaseOrders(db *gorm.DB) *PurchaseOrders { return &PurchaseOrders{db: db} }

func (p *PurchaseOrders) ListPurchaseOrders(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.ListPurchaseOrdersRequest],
) (*connect.Response[purchasingifacev1.ListPurchaseOrdersResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, p.db, caller)
	if err != nil {
		return nil, err
	}
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Where("warehouse_id = ?", warehouseID)
		if statusStr := poStatusToString(req.Msg.Status); statusStr != "" {
			q = q.Where("status = ?", statusStr)
		}
		if req.Msg.SupplierId != "" {
			q = q.Where("supplier_id = ?", req.Msg.SupplierId)
		}
		if req.Msg.OnlyOutstanding {
			q = q.Where("status NOT IN ?", []string{poStatusVoided, poStatusDraft}).
				Where("ordered_total > paid_amount")
		}
		// Free-text search: PO number, supplier name/code, or any ordered medicine.
		if query := strings.TrimSpace(req.Msg.Query); query != "" {
			pattern := "%" + query + "%"
			sub := p.db.Table("purchase_orders AS po").
				Select("po.id").
				Joins("JOIN suppliers s ON s.id = po.supplier_id").
				Joins("LEFT JOIN purchase_order_items poi ON poi.purchase_order_id = po.id").
				Joins("LEFT JOIN medicines m ON m.id = poi.medicine_id").
				Where("po.po_no ILIKE ? OR s.name ILIKE ? OR s.code ILIKE ? OR m.name ILIKE ?",
					pattern, pattern, pattern, pattern)
			q = q.Where("id IN (?)", sub)
		}
		// Date range over the created date or the (latest) received date.
		if req.Msg.FromUnix > 0 || req.Msg.ToUnix > 0 {
			if req.Msg.DateField == "received" {
				sub := p.db.Table("purchase_receipts").Select("purchase_order_id")
				if req.Msg.FromUnix > 0 {
					sub = sub.Where("received_at >= ?", time.Unix(req.Msg.FromUnix, 0))
				}
				if req.Msg.ToUnix > 0 {
					sub = sub.Where("received_at < ?", time.Unix(req.Msg.ToUnix, 0))
				}
				q = q.Where("id IN (?)", sub)
			} else {
				if req.Msg.FromUnix > 0 {
					q = q.Where("created_at >= ?", time.Unix(req.Msg.FromUnix, 0))
				}
				if req.Msg.ToUnix > 0 {
					q = q.Where("created_at < ?", time.Unix(req.Msg.ToUnix, 0))
				}
			}
		}
		return q
	}

	var total int64
	if err := applyFilters(p.db.WithContext(ctx).Model(&model.PurchaseOrder{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var rows []model.PurchaseOrder
	if err := applyFilters(p.db.WithContext(ctx).Preload("Items")).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*purchasingifacev1.PurchaseOrder, 0, len(rows))
	for i := range rows {
		out = append(out, poToProto(&rows[i]))
	}
	if err := p.enrichList(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.ListPurchaseOrdersResponse{
		Orders: out,
		Total:  int32(total),
	}), nil
}

// enrichList denormalizes display-only data onto a page of POs: medicine names
// for each ordered item, plus the most recent receipt's date + invoice number.
// Batched (two queries, bounded by the page limit) to avoid N+1.
func (p *PurchaseOrders) enrichList(ctx context.Context, orders []*purchasingifacev1.PurchaseOrder) error {
	if len(orders) == 0 {
		return nil
	}
	poIDs := make([]string, 0, len(orders))
	medIDSet := map[string]struct{}{}
	for _, po := range orders {
		poIDs = append(poIDs, po.Id)
		for _, it := range po.Items {
			if it.MedicineId != "" {
				medIDSet[it.MedicineId] = struct{}{}
			}
		}
	}

	// Medicine names + SKUs.
	if len(medIDSet) > 0 {
		medIDs := make([]string, 0, len(medIDSet))
		for id := range medIDSet {
			medIDs = append(medIDs, id)
		}
		type nameRow struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
			Sku  string `gorm:"column:sku"`
		}
		var names []nameRow
		if err := p.db.WithContext(ctx).Table("medicines").
			Select("id, name, sku").Where("id IN ?", medIDs).Scan(&names).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		nameByID := make(map[string]string, len(names))
		skuByID := make(map[string]string, len(names))
		for _, n := range names {
			nameByID[n.ID] = n.Name
			skuByID[n.ID] = n.Sku
		}
		for _, po := range orders {
			for _, it := range po.Items {
				it.MedicineName = nameByID[it.MedicineId]
				it.MedicineSku = skuByID[it.MedicineId]
			}
		}
	}

	// Most recent receipt per PO (received_at + invoice_no).
	type rcvRow struct {
		PurchaseOrderID string    `gorm:"column:purchase_order_id"`
		ReceivedAt      time.Time `gorm:"column:received_at"`
		InvoiceNo       string    `gorm:"column:invoice_no"`
	}
	var rcvs []rcvRow
	if err := p.db.WithContext(ctx).
		Raw(`SELECT DISTINCT ON (purchase_order_id)
		        purchase_order_id, received_at, invoice_no
		     FROM purchase_receipts
		     WHERE purchase_order_id IN ?
		     ORDER BY purchase_order_id, received_at DESC, created_at DESC`, poIDs).
		Scan(&rcvs).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	type rcvInfo struct {
		at  int64
		inv string
	}
	rcvByPO := make(map[string]rcvInfo, len(rcvs))
	for _, r := range rcvs {
		rcvByPO[r.PurchaseOrderID] = rcvInfo{at: r.ReceivedAt.Unix(), inv: r.InvoiceNo}
	}
	for _, po := range orders {
		if info, ok := rcvByPO[po.Id]; ok {
			po.ReceivedAt = info.at
			// Only fall back to the latest receipt's faktur when the PO
			// has no faktur of its own (legacy rows pre-00027).
			if po.InvoiceNo == "" {
				po.InvoiceNo = info.inv
			}
		}
	}

	// Warehouse names (one row per PO; all rows usually share one warehouse but
	// GetPurchaseOrder also goes through this path with a single row).
	whIDSet := map[string]struct{}{}
	for _, po := range orders {
		if po.WarehouseId != "" {
			whIDSet[po.WarehouseId] = struct{}{}
		}
	}
	if len(whIDSet) > 0 {
		whIDs := make([]string, 0, len(whIDSet))
		for id := range whIDSet {
			whIDs = append(whIDs, id)
		}
		type whRow struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		var rows []whRow
		if err := p.db.WithContext(ctx).Table("warehouses").
			Select("id, name").Where("id IN ?", whIDs).Scan(&rows).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		byID := make(map[string]string, len(rows))
		for _, r := range rows {
			byID[r.ID] = r.Name
		}
		for _, po := range orders {
			po.WarehouseName = byID[po.WarehouseId]
		}
	}
	return nil
}

func (p *PurchaseOrders) GetPurchaseOrder(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.GetPurchaseOrderRequest],
) (*connect.Response[purchasingifacev1.GetPurchaseOrderResponse], error) {
	po, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.GetPurchaseOrderResponse{Order: poToProto(po)}), nil
}

func (p *PurchaseOrders) CreatePurchaseOrder(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.CreatePurchaseOrderRequest],
) (*connect.Response[purchasingifacev1.CreatePurchaseOrderResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.SupplierId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("supplier_id required"))
	}
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item required"))
	}

	warehouseID, err := resolveWarehouse(ctx, p.db, caller)
	if err != nil {
		return nil, err
	}

	var po model.PurchaseOrder
	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po = model.PurchaseOrder{
			SupplierID:  req.Msg.SupplierId,
			Status:      poStatusDraft,
			Note:        strings.TrimSpace(req.Msg.Note),
			InvoiceNo:   strings.TrimSpace(req.Msg.InvoiceNo),
			CreatedBy:   caller.UserID,
			WarehouseID: warehouseID,
			PpnEnabled:  req.Msg.PpnEnabled,
		}
		if e, err := parseDateMaybe(req.Msg.InvoiceDate); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		} else if e != nil {
			po.InvoiceDate = e
		}
		if e, err := parseDateMaybe(req.Msg.DueAt); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		} else if e != nil {
			po.DueAt = e
		}

		var items []model.PurchaseOrderItem
		for _, in := range req.Msg.Items {
			if in.OrderedQty <= 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("ordered_qty must be > 0"))
			}
			if in.UnitCostPrice < 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("unit_cost_price must be >= 0"))
			}
			unit, err := resolvePurchaseUnit(tx, in.MedicineId, in.MedicineUnitId)
			if err != nil {
				return err
			}
			baseQty := in.OrderedQty * int32(unit.Factor) // ordered_qty stored in BASE units
			it := model.PurchaseOrderItem{
				MedicineID:     in.MedicineId,
				OrderedQty:     baseQty,
				UnitCostPrice:  in.UnitCostPrice, // per base unit
				Subtotal:       int64(baseQty) * in.UnitCostPrice,
				MedicineUnitID: &unit.ID,
				UnitName:       unit.Name,
				UnitFactor:     unit.Factor,
			}
			items = append(items, it)
		}
		subtotal, discount, rate, ppn, total := computePOTotals(items, req.Msg.CartDiscount, req.Msg.PpnEnabled, req.Msg.PpnRate)
		po.Subtotal = subtotal
		po.CartDiscount = discount
		po.PpnRate = rate
		po.PpnAmount = ppn
		po.OrderedTotal = total

		// Assign PO number up-front (per-year sequence).
		poNo, err := assignPONo(tx, time.Now())
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		po.PoNo = &poNo

		if err := tx.Create(&po).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		for i := range items {
			items[i].PurchaseOrderID = po.ID
		}
		if err := tx.Create(&items).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}

	full, err := p.loadFull(ctx, po.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.CreatePurchaseOrderResponse{Order: poToProto(full)}), nil
}

func (p *PurchaseOrders) UpdatePurchaseOrder(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.UpdatePurchaseOrderRequest],
) (*connect.Response[purchasingifacev1.UpdatePurchaseOrderResponse], error) {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po, err := p.lockByID(tx, req.Msg.Id)
		if err != nil {
			return err
		}
		if po.Status != poStatusDraft {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only DRAFT POs are editable; this one is %s", po.Status))
		}
		updates := map[string]any{
			"note":         strings.TrimSpace(req.Msg.Note),
			"invoice_no":   strings.TrimSpace(req.Msg.InvoiceNo),
			"ppn_enabled":  req.Msg.PpnEnabled,
		}
		if e, err := parseDateMaybe(req.Msg.InvoiceDate); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		} else {
			updates["invoice_date"] = e
		}
		if e, err := parseDateMaybe(req.Msg.DueAt); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		} else {
			updates["due_at"] = e
		}
		if err := tx.Model(po).Updates(updates).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Recompute totals: either with the new items (if provided) or with the
		// existing items reloaded from DB. Either way the PPN/discount inputs
		// from the request drive the math.
		var items []model.PurchaseOrderItem
		if len(req.Msg.Items) > 0 {
			if err := tx.Where("purchase_order_id = ?", po.ID).Delete(&model.PurchaseOrderItem{}).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			for _, in := range req.Msg.Items {
				if in.OrderedQty <= 0 {
					return connect.NewError(connect.CodeInvalidArgument, errors.New("ordered_qty must be > 0"))
				}
				unit, err := resolvePurchaseUnit(tx, in.MedicineId, in.MedicineUnitId)
				if err != nil {
					return err
				}
				baseQty := in.OrderedQty * int32(unit.Factor) // ordered_qty stored in BASE units
				it := model.PurchaseOrderItem{
					PurchaseOrderID: po.ID,
					MedicineID:      in.MedicineId,
					OrderedQty:      baseQty,
					UnitCostPrice:   in.UnitCostPrice, // per base unit
					Subtotal:        int64(baseQty) * in.UnitCostPrice,
					MedicineUnitID:  &unit.ID,
					UnitName:        unit.Name,
					UnitFactor:      unit.Factor,
				}
				items = append(items, it)
			}
			if err := tx.Create(&items).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		} else {
			if err := tx.Where("purchase_order_id = ?", po.ID).Find(&items).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
		subtotal, discount, rate, ppn, total := computePOTotals(items, req.Msg.CartDiscount, req.Msg.PpnEnabled, req.Msg.PpnRate)
		if err := tx.Model(po).Updates(map[string]any{
			"subtotal":      subtotal,
			"cart_discount": discount,
			"ppn_rate":      rate,
			"ppn_amount":    ppn,
			"ordered_total": total,
		}).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}

	full, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.UpdatePurchaseOrderResponse{Order: poToProto(full)}), nil
}

func (p *PurchaseOrders) SendPurchaseOrder(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.SendPurchaseOrderRequest],
) (*connect.Response[purchasingifacev1.SendPurchaseOrderResponse], error) {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po, err := p.lockByID(tx, req.Msg.Id)
		if err != nil {
			return err
		}
		if po.Status != poStatusDraft {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only DRAFT POs can be sent; this one is %s", po.Status))
		}
		now := time.Now()
		return tx.Model(po).Updates(map[string]any{
			"status":  poStatusSent,
			"sent_at": now,
		}).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	full, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.SendPurchaseOrderResponse{Order: poToProto(full)}), nil
}

func (p *PurchaseOrders) VoidPurchaseOrder(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.VoidPurchaseOrderRequest],
) (*connect.Response[purchasingifacev1.VoidPurchaseOrderResponse], error) {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po, err := p.lockByID(tx, req.Msg.Id)
		if err != nil {
			return err
		}
		if po.Status != poStatusDraft && po.Status != poStatusSent {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only DRAFT or SENT POs can be voided; this one is %s", po.Status))
		}
		return tx.Model(po).Update("status", poStatusVoided).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	full, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.VoidPurchaseOrderResponse{Order: poToProto(full)}), nil
}

// ---------- Helpers (also used by receipts.go + payments.go) ----------

func (p *PurchaseOrders) lockByID(tx *gorm.DB, id string) (*model.PurchaseOrder, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var po model.PurchaseOrder
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("purchase order not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &po, nil
}

func (p *PurchaseOrders) loadFull(ctx context.Context, id string) (*model.PurchaseOrder, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var po model.PurchaseOrder
	err := p.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("purchase order not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &po, nil
}

func parseDateMaybe(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("date must be YYYY-MM-DD: %w", err)
	}
	return &t, nil
}

func assignPONo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	var counter model.POCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.POCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := tx.Model(&model.POCounter{}).
		Where("year = ?", year).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("PO-%d-%04d", year, counter.LastSeq), nil
}

func assignReceiptNo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	var counter model.RcvCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.RcvCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := tx.Model(&model.RcvCounter{}).
		Where("year = ?", year).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("RCV-%d-%04d", year, counter.LastSeq), nil
}

// recomputePOStatus inspects items and bumps po.status accordingly. Caller is
// inside a tx and the po row is locked.
func recomputePOStatus(tx *gorm.DB, po *model.PurchaseOrder) error {
	var items []model.PurchaseOrderItem
	if err := tx.Where("purchase_order_id = ?", po.ID).Find(&items).Error; err != nil {
		return err
	}
	allReceived := true
	anyReceived := false
	for _, it := range items {
		if it.ReceivedQty < it.OrderedQty {
			allReceived = false
		}
		if it.ReceivedQty > 0 {
			anyReceived = true
		}
	}
	var newStatus string
	switch {
	case allReceived && anyReceived:
		newStatus = poStatusReceived
	case anyReceived:
		newStatus = poStatusPartiallyReceived
	default:
		// no receipts yet, keep current
		return nil
	}
	// If already CLOSED (paid in full), don't downgrade.
	if po.Status == poStatusClosed {
		return nil
	}
	po.Status = newStatus
	return tx.Model(po).Update("status", newStatus).Error
}

// maybeCloseIfPaid: when paid >= ordered_total AND status == RECEIVED, mark CLOSED.
func maybeCloseIfPaid(tx *gorm.DB, po *model.PurchaseOrder) error {
	if po.Status == poStatusReceived && po.PaidAmount >= po.OrderedTotal && po.OrderedTotal > 0 {
		now := time.Now()
		po.Status = poStatusClosed
		po.ClosedAt = &now
		return tx.Model(po).Updates(map[string]any{
			"status":    poStatusClosed,
			"closed_at": now,
		}).Error
	}
	return nil
}

// resolvePurchaseUnit returns the medicine's purchasable unit for the given unit
// id, or its base unit when unitID is empty. Errors if the unit isn't
// purchasable/active or doesn't belong to the medicine. Mirrors resolveSellUnit.
func resolvePurchaseUnit(tx *gorm.DB, medicineID, unitID string) (*model.MedicineUnit, error) {
	var u model.MedicineUnit
	q := tx.Where("medicine_id = ? AND active", medicineID)
	if unitID != "" {
		q = q.Where("id = ? AND purchasable", unitID)
	} else {
		q = q.Where("is_base")
	}
	if err := q.First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("purchase unit not found or not purchasable"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if u.Factor < 1 {
		u.Factor = 1
	}
	return &u, nil
}

// ---------- Proto mapping ----------

func poToProto(po *model.PurchaseOrder) *purchasingifacev1.PurchaseOrder {
	out := &purchasingifacev1.PurchaseOrder{
		Id:           po.ID,
		SupplierId:   po.SupplierID,
		Status:       poStatusFromString(po.Status),
		Note:         po.Note,
		InvoiceNo:    po.InvoiceNo,
		Subtotal:     po.Subtotal,
		CartDiscount: po.CartDiscount,
		PpnEnabled:   po.PpnEnabled,
		PpnRate:      po.PpnRate,
		PpnAmount:    po.PpnAmount,
		OrderedTotal: po.OrderedTotal,
		PaidAmount:   po.PaidAmount,
		Outstanding:  po.OrderedTotal - po.PaidAmount,
		CreatedBy:    po.CreatedBy,
		CreatedAt:    po.CreatedAt.Unix(),
		WarehouseId:  po.WarehouseID,
	}
	if po.PoNo != nil {
		out.PoNo = *po.PoNo
	}
	if po.InvoiceDate != nil {
		out.InvoiceDate = po.InvoiceDate.Format("2006-01-02")
	}
	if po.DueAt != nil {
		out.DueAt = po.DueAt.Format("2006-01-02")
	}
	if po.BranchID != nil {
		out.BranchId = *po.BranchID
	}
	if po.SentAt != nil {
		out.SentAt = po.SentAt.Unix()
	}
	if po.ClosedAt != nil {
		out.ClosedAt = po.ClosedAt.Unix()
	}
	for i := range po.Items {
		out.Items = append(out.Items, poItemToProto(&po.Items[i]))
	}
	return out
}

func poItemToProto(it *model.PurchaseOrderItem) *purchasingifacev1.PurchaseOrderItem {
	factor := it.UnitFactor
	if factor < 1 {
		factor = 1
	}
	out := &purchasingifacev1.PurchaseOrderItem{
		Id:              it.ID,
		PurchaseOrderId: it.PurchaseOrderID,
		MedicineId:      it.MedicineID,
		OrderedQty:      it.OrderedQty,
		ReceivedQty:     it.ReceivedQty,
		UnitCostPrice:   it.UnitCostPrice,
		Subtotal:        it.Subtotal,
		UnitName:        it.UnitName,
		UnitFactor:      factor,
	}
	if it.MedicineUnitID != nil {
		out.MedicineUnitId = *it.MedicineUnitID
	}
	return out
}

func poStatusToString(s purchasingifacev1.POStatus) string {
	switch s {
	case purchasingifacev1.POStatus_PO_STATUS_DRAFT:
		return poStatusDraft
	case purchasingifacev1.POStatus_PO_STATUS_SENT:
		return poStatusSent
	case purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED:
		return poStatusPartiallyReceived
	case purchasingifacev1.POStatus_PO_STATUS_RECEIVED:
		return poStatusReceived
	case purchasingifacev1.POStatus_PO_STATUS_CLOSED:
		return poStatusClosed
	case purchasingifacev1.POStatus_PO_STATUS_VOIDED:
		return poStatusVoided
	default:
		return ""
	}
}

func poStatusFromString(s string) purchasingifacev1.POStatus {
	switch s {
	case poStatusDraft:
		return purchasingifacev1.POStatus_PO_STATUS_DRAFT
	case poStatusSent:
		return purchasingifacev1.POStatus_PO_STATUS_SENT
	case poStatusPartiallyReceived:
		return purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED
	case poStatusReceived:
		return purchasingifacev1.POStatus_PO_STATUS_RECEIVED
	case poStatusClosed:
		return purchasingifacev1.POStatus_PO_STATUS_CLOSED
	case poStatusVoided:
		return purchasingifacev1.POStatus_PO_STATUS_VOIDED
	default:
		return purchasingifacev1.POStatus_PO_STATUS_UNSPECIFIED
	}
}
