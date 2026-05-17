package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/apotech/backend/gen/purchasing_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
)

type PurchaseReceipts struct {
	db *gorm.DB
}

func NewPurchaseReceipts(db *gorm.DB) *PurchaseReceipts { return &PurchaseReceipts{db: db} }

func (p *PurchaseReceipts) CreateReceipt(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.CreateReceiptRequest],
) (*connect.Response[purchasingifacev1.CreateReceiptResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.PurchaseOrderId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("purchase_order_id required"))
	}
	if len(req.Msg.Lines) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one line required"))
	}

	// Default receipt date to today.
	receivedAt := time.Now()
	if req.Msg.ReceivedAt != "" {
		t, err := time.Parse("2006-01-02", req.Msg.ReceivedAt)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("received_at must be YYYY-MM-DD: %w", err))
		}
		receivedAt = t
	}

	var receipt model.PurchaseReceipt

	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the PO; reject if not in a receivable state.
		var po model.PurchaseOrder
		if err := tx.Where("id = ?", req.Msg.PurchaseOrderId).First(&po).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("purchase order not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		switch po.Status {
		case poStatusSent, poStatusPartiallyReceived:
			// receivable — proceed
		default:
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("cannot receive a PO in status %s; send it first", po.Status))
		}

		// Create the receipt header with an assigned receipt_no.
		receiptNo, err := assignReceiptNo(tx, time.Now())
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		receipt = model.PurchaseReceipt{
			ReceiptNo:       &receiptNo,
			PurchaseOrderID: po.ID,
			ReceivedAt:      receivedAt,
			ReceivedBy:      caller.UserID,
			Note:            strings.TrimSpace(req.Msg.Note),
		}
		if err := tx.Create(&receipt).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Process each line: load PO item, validate qty, create batch + stock_movement.
		for _, line := range req.Msg.Lines {
			if line.Qty <= 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("qty must be > 0"))
			}
			expiry, err := time.Parse("2006-01-02", line.ExpiryDate)
			if err != nil {
				return connect.NewError(connect.CodeInvalidArgument,
					fmt.Errorf("expiry_date must be YYYY-MM-DD: %w", err))
			}

			var poItem model.PurchaseOrderItem
			err = tx.Where("id = ? AND purchase_order_id = ?",
				line.PurchaseOrderItemId, po.ID).First(&poItem).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeInvalidArgument,
					errors.New("purchase_order_item not found on this PO"))
			}
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			remaining := poItem.OrderedQty - poItem.ReceivedQty
			if line.Qty > remaining {
				return connect.NewError(connect.CodeFailedPrecondition,
					fmt.Errorf("line qty %d exceeds remaining %d for medicine %s",
						line.Qty, remaining, poItem.MedicineID))
			}

			unitCost := line.UnitCostPrice
			if unitCost == 0 {
				unitCost = poItem.UnitCostPrice
			}

			// Create the batch row carrying supplier + cost + expiry.
			supplierID := po.SupplierID
			batch := model.Batch{
				MedicineID:  poItem.MedicineID,
				SupplierID:  &supplierID,
				BatchNumber: strings.TrimSpace(line.BatchNumber),
				ExpiryDate:  expiry,
				CostPrice:   unitCost,
				ReceivedAt:  receivedAt,
			}
			if err := tx.Create(&batch).Error; err != nil {
				return connect.NewError(connect.CodeInternal,
					fmt.Errorf("create batch: %w", err))
			}

			// PURCHASE stock_movement linked to this batch.
			mv := model.StockMovement{
				BatchID: batch.ID,
				Qty:     line.Qty,
				Type:    "PURCHASE",
				Reason:  fmt.Sprintf("PO %s receipt %s", deref(po.PoNo), receiptNo),
				UserID:  caller.UserID,
			}
			if err := tx.Create(&mv).Error; err != nil {
				return connect.NewError(connect.CodeInternal,
					fmt.Errorf("create stock movement: %w", err))
			}

			// Receipt item row linking back to the batch we just created.
			batchID := batch.ID
			rcvItem := model.PurchaseReceiptItem{
				PurchaseReceiptID:   receipt.ID,
				PurchaseOrderItemID: poItem.ID,
				MedicineID:          poItem.MedicineID,
				Qty:                 line.Qty,
				UnitCostPrice:       unitCost,
				BatchNumber:         strings.TrimSpace(line.BatchNumber),
				ExpiryDate:          expiry,
				BatchID:             &batchID,
			}
			if err := tx.Create(&rcvItem).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			// Bump received_qty on the PO item.
			if err := tx.Model(&model.PurchaseOrderItem{}).
				Where("id = ?", poItem.ID).
				Update("received_qty", gorm.Expr("received_qty + ?", line.Qty)).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		// Recompute PO status now that received_qty has changed.
		if err := recomputePOStatus(tx, &po); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}

	full, err := p.loadFull(ctx, receipt.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.CreateReceiptResponse{Receipt: receiptToProto(full)}), nil
}

func (p *PurchaseReceipts) ListReceipts(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.ListReceiptsRequest],
) (*connect.Response[purchasingifacev1.ListReceiptsResponse], error) {
	q := p.db.WithContext(ctx).Preload("Items").Order("created_at DESC")
	if req.Msg.PurchaseOrderId != "" {
		q = q.Where("purchase_order_id = ?", req.Msg.PurchaseOrderId)
	}
	var rows []model.PurchaseReceipt
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*purchasingifacev1.PurchaseReceipt, 0, len(rows))
	for i := range rows {
		out = append(out, receiptToProto(&rows[i]))
	}
	return connect.NewResponse(&purchasingifacev1.ListReceiptsResponse{Receipts: out}), nil
}

func (p *PurchaseReceipts) GetReceipt(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.GetReceiptRequest],
) (*connect.Response[purchasingifacev1.GetReceiptResponse], error) {
	r, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.GetReceiptResponse{Receipt: receiptToProto(r)}), nil
}

func (p *PurchaseReceipts) loadFull(ctx context.Context, id string) (*model.PurchaseReceipt, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var r model.PurchaseReceipt
	err := p.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("receipt not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &r, nil
}

func receiptToProto(r *model.PurchaseReceipt) *purchasingifacev1.PurchaseReceipt {
	out := &purchasingifacev1.PurchaseReceipt{
		Id:              r.ID,
		PurchaseOrderId: r.PurchaseOrderID,
		ReceivedAt:      r.ReceivedAt.Format("2006-01-02"),
		ReceivedBy:      r.ReceivedBy,
		Note:            r.Note,
		CreatedAt:       r.CreatedAt.Unix(),
	}
	if r.ReceiptNo != nil {
		out.ReceiptNo = *r.ReceiptNo
	}
	for i := range r.Items {
		out.Items = append(out.Items, receiptItemToProto(&r.Items[i]))
	}
	return out
}

func receiptItemToProto(it *model.PurchaseReceiptItem) *purchasingifacev1.PurchaseReceiptItem {
	out := &purchasingifacev1.PurchaseReceiptItem{
		Id:                  it.ID,
		PurchaseReceiptId:   it.PurchaseReceiptID,
		PurchaseOrderItemId: it.PurchaseOrderItemID,
		MedicineId:          it.MedicineID,
		Qty:                 it.Qty,
		UnitCostPrice:       it.UnitCostPrice,
		BatchNumber:         it.BatchNumber,
		ExpiryDate:          it.ExpiryDate.Format("2006-01-02"),
	}
	if it.BatchID != nil {
		out.BatchId = *it.BatchID
	}
	return out
}
