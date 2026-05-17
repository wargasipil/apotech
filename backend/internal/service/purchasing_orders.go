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
)

type PurchaseOrders struct {
	db *gorm.DB
}

func NewPurchaseOrders(db *gorm.DB) *PurchaseOrders { return &PurchaseOrders{db: db} }

func (p *PurchaseOrders) ListPurchaseOrders(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.ListPurchaseOrdersRequest],
) (*connect.Response[purchasingifacev1.ListPurchaseOrdersResponse], error) {
	q := p.db.WithContext(ctx).Preload("Items").Order("created_at DESC")
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
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q = q.Limit(limit)

	var rows []model.PurchaseOrder
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*purchasingifacev1.PurchaseOrder, 0, len(rows))
	for i := range rows {
		out = append(out, poToProto(&rows[i]))
	}
	return connect.NewResponse(&purchasingifacev1.ListPurchaseOrdersResponse{Orders: out}), nil
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

	var po model.PurchaseOrder
	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po = model.PurchaseOrder{
			SupplierID: req.Msg.SupplierId,
			Status:     poStatusDraft,
			Note:       strings.TrimSpace(req.Msg.Note),
			CreatedBy:  caller.UserID,
		}
		if e, err := parseDateMaybe(req.Msg.ExpectedAt); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		} else if e != nil {
			po.ExpectedAt = e
		}

		var subtotal int64
		var items []model.PurchaseOrderItem
		for _, in := range req.Msg.Items {
			if in.OrderedQty <= 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("ordered_qty must be > 0"))
			}
			if in.UnitCostPrice < 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("unit_cost_price must be >= 0"))
			}
			it := model.PurchaseOrderItem{
				MedicineID:    in.MedicineId,
				OrderedQty:    in.OrderedQty,
				UnitCostPrice: in.UnitCostPrice,
				Subtotal:      int64(in.OrderedQty) * in.UnitCostPrice,
			}
			subtotal += it.Subtotal
			items = append(items, it)
		}
		po.OrderedTotal = subtotal

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
		updates := map[string]any{"note": strings.TrimSpace(req.Msg.Note)}
		if e, err := parseDateMaybe(req.Msg.ExpectedAt); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		} else {
			updates["expected_at"] = e
		}
		if err := tx.Model(po).Updates(updates).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Full replace of items only if provided. Empty items list = leave alone.
		if len(req.Msg.Items) > 0 {
			if err := tx.Where("purchase_order_id = ?", po.ID).Delete(&model.PurchaseOrderItem{}).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			var subtotal int64
			var items []model.PurchaseOrderItem
			for _, in := range req.Msg.Items {
				if in.OrderedQty <= 0 {
					return connect.NewError(connect.CodeInvalidArgument, errors.New("ordered_qty must be > 0"))
				}
				it := model.PurchaseOrderItem{
					PurchaseOrderID: po.ID,
					MedicineID:      in.MedicineId,
					OrderedQty:      in.OrderedQty,
					UnitCostPrice:   in.UnitCostPrice,
					Subtotal:        int64(in.OrderedQty) * in.UnitCostPrice,
				}
				subtotal += it.Subtotal
				items = append(items, it)
			}
			if err := tx.Create(&items).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			if err := tx.Model(po).Update("ordered_total", subtotal).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
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

// ---------- Proto mapping ----------

func poToProto(po *model.PurchaseOrder) *purchasingifacev1.PurchaseOrder {
	out := &purchasingifacev1.PurchaseOrder{
		Id:           po.ID,
		SupplierId:   po.SupplierID,
		Status:       poStatusFromString(po.Status),
		Note:         po.Note,
		OrderedTotal: po.OrderedTotal,
		PaidAmount:   po.PaidAmount,
		Outstanding:  po.OrderedTotal - po.PaidAmount,
		CreatedBy:    po.CreatedBy,
		CreatedAt:    po.CreatedAt.Unix(),
	}
	if po.PoNo != nil {
		out.PoNo = *po.PoNo
	}
	if po.ExpectedAt != nil {
		out.ExpectedAt = po.ExpectedAt.Format("2006-01-02")
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
	return &purchasingifacev1.PurchaseOrderItem{
		Id:              it.ID,
		PurchaseOrderId: it.PurchaseOrderID,
		MedicineId:      it.MedicineID,
		OrderedQty:      it.OrderedQty,
		ReceivedQty:     it.ReceivedQty,
		UnitCostPrice:   it.UnitCostPrice,
		Subtotal:        it.Subtotal,
	}
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
