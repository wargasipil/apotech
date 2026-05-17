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

	posifacev1 "github.com/apotech/backend/gen/pos_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/config"
	"github.com/apotech/backend/internal/model"
	"github.com/apotech/backend/internal/printer"
)

const (
	saleStatusDraft     = "DRAFT"
	saleStatusCompleted = "COMPLETED"
	saleStatusVoided    = "VOIDED"

	paymentCash             = "CASH"
	paymentBPJS             = "BPJS"
	paymentInsuranceOther   = "INSURANCE_OTHER"

	movementTypeSale = "SALE"
)

type Sales struct {
	db      *gorm.DB
	printer config.Printer
}

func NewSales(db *gorm.DB, printerCfg config.Printer) *Sales {
	return &Sales{db: db, printer: printerCfg}
}

// ---------- Lifecycle ----------

func (s *Sales) StartSale(
	ctx context.Context,
	_ *connect.Request[posifacev1.StartSaleRequest],
) (*connect.Response[posifacev1.StartSaleResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sale := model.Sale{
		CashierUserID: caller.UserID,
		Status:        saleStatusDraft,
	}
	if caller.BranchID != "" {
		bid := caller.BranchID
		sale.BranchID = &bid
	}
	if err := s.db.WithContext(ctx).Create(&sale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out, err := s.loadFull(ctx, sale.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.StartSaleResponse{Sale: saleToProto(out)}), nil
}

func (s *Sales) GetSale(
	ctx context.Context,
	req *connect.Request[posifacev1.GetSaleRequest],
) (*connect.Response[posifacev1.GetSaleResponse], error) {
	sale, err := s.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.GetSaleResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) ListSales(
	ctx context.Context,
	req *connect.Request[posifacev1.ListSalesRequest],
) (*connect.Response[posifacev1.ListSalesResponse], error) {
	q := s.db.WithContext(ctx).Preload("Items").Order("created_at DESC")
	if req.Msg.FromUnix > 0 {
		q = q.Where("created_at >= ?", time.Unix(req.Msg.FromUnix, 0))
	}
	if req.Msg.ToUnix > 0 {
		q = q.Where("created_at < ?", time.Unix(req.Msg.ToUnix, 0))
	}
	if statusStr := saleStatusToString(req.Msg.Status); statusStr != "" {
		q = q.Where("status = ?", statusStr)
	}
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q = q.Limit(limit)

	var rows []model.Sale
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*posifacev1.Sale, 0, len(rows))
	for i := range rows {
		out = append(out, saleToProto(&rows[i]))
	}
	return connect.NewResponse(&posifacev1.ListSalesResponse{Sales: out}), nil
}

// ---------- Item ops ----------

func (s *Sales) AddItem(
	ctx context.Context,
	req *connect.Request[posifacev1.AddItemRequest],
) (*connect.Response[posifacev1.AddItemResponse], error) {
	if req.Msg.Qty <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("qty must be > 0"))
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}

		// Look up medicine to snapshot the current price.
		var med model.Medicine
		if err := tx.Where("id = ?", req.Msg.MedicineId).First(&med).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, fmt.Errorf("medicine %s not found", req.Msg.MedicineId))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if !med.Active {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("medicine is archived"))
		}

		// Look up the existing line for this medicine (qty after this add).
		var existing model.SaleItem
		findErr := tx.Where("sale_id = ? AND medicine_id = ?", sale.ID, med.ID).First(&existing).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return connect.NewError(connect.CodeInternal, findErr)
		}
		newQty := req.Msg.Qty
		if findErr == nil {
			newQty = existing.Qty + req.Msg.Qty
		}

		// Rx gate: prescription_required medicines need a valid Rx attached
		// with sufficient remaining qty.
		if med.PrescriptionRequired {
			if err := assertRxCovers(tx, sale, med.ID, newQty); err != nil {
				return err
			}
		}

		// If an item for this medicine already exists, increment its qty.
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			item := model.SaleItem{
				SaleID:            sale.ID,
				MedicineID:        med.ID,
				Qty:               req.Msg.Qty,
				UnitPriceSnapshot: med.UnitPrice,
			}
			item.LineTotal = computeLineTotal(item.Qty, item.UnitPriceSnapshot, item.LineDiscount)
			if err := tx.Create(&item).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		} else {
			existing.Qty = newQty
			existing.UnitPriceSnapshot = med.UnitPrice
			existing.LineTotal = computeLineTotal(existing.Qty, existing.UnitPriceSnapshot, existing.LineDiscount)
			if err := tx.Save(&existing).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		return recomputeSaleTotals(tx, sale.ID)
	})
	if err != nil {
		return nil, asConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.AddItemResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) SetItemQuantity(
	ctx context.Context,
	req *connect.Request[posifacev1.SetItemQuantityRequest],
) (*connect.Response[posifacev1.SetItemQuantityResponse], error) {
	if req.Msg.Qty <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("qty must be > 0"))
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		var item model.SaleItem
		if err := tx.Where("id = ? AND sale_id = ?", req.Msg.ItemId, sale.ID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("item not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		var med model.Medicine
		if err := tx.Where("id = ?", item.MedicineID).First(&med).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if med.PrescriptionRequired {
			if err := assertRxCovers(tx, sale, med.ID, req.Msg.Qty); err != nil {
				return err
			}
		}
		item.Qty = req.Msg.Qty
		item.LineTotal = computeLineTotal(item.Qty, item.UnitPriceSnapshot, item.LineDiscount)
		if err := tx.Save(&item).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return recomputeSaleTotals(tx, sale.ID)
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.SetItemQuantityResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) RemoveItem(
	ctx context.Context,
	req *connect.Request[posifacev1.RemoveItemRequest],
) (*connect.Response[posifacev1.RemoveItemResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		res := tx.Where("id = ? AND sale_id = ?", req.Msg.ItemId, sale.ID).Delete(&model.SaleItem{})
		if res.Error != nil {
			return connect.NewError(connect.CodeInternal, res.Error)
		}
		if res.RowsAffected == 0 {
			return connect.NewError(connect.CodeNotFound, errors.New("item not found"))
		}
		return recomputeSaleTotals(tx, sale.ID)
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.RemoveItemResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) SetSaleCustomer(
	ctx context.Context,
	req *connect.Request[posifacev1.SetSaleCustomerRequest],
) (*connect.Response[posifacev1.SetSaleCustomerResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		updates := map[string]any{"customer_id": nil}
		if req.Msg.CustomerId != "" {
			updates["customer_id"] = req.Msg.CustomerId
		}
		return tx.Model(sale).Updates(updates).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.SetSaleCustomerResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) AttachPrescription(
	ctx context.Context,
	req *connect.Request[posifacev1.AttachPrescriptionRequest],
) (*connect.Response[posifacev1.AttachPrescriptionResponse], error) {
	if req.Msg.PrescriptionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("prescription_id required"))
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		var rx model.Prescription
		if err := tx.Preload("Items").Where("id = ?", req.Msg.PrescriptionId).First(&rx).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("prescription not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		switch computeRxStatus(&rx, time.Now()) {
		case rxStatusActive:
			// ok
		case rxStatusDispensed:
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("prescription already fully dispensed"))
		case rxStatusExpired:
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("prescription has expired"))
		case rxStatusVoided:
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("prescription is voided"))
		}
		// If a customer is already set on the sale, it must match the Rx customer.
		if sale.CustomerID != nil && *sale.CustomerID != "" && *sale.CustomerID != rx.CustomerID {
			return connect.NewError(connect.CodeFailedPrecondition,
				errors.New("sale customer does not match prescription customer"))
		}
		updates := map[string]any{"prescription_id": rx.ID}
		// If sale has no customer, snap to the Rx's customer for convenience.
		if sale.CustomerID == nil || *sale.CustomerID == "" {
			updates["customer_id"] = rx.CustomerID
		}
		return tx.Model(sale).Updates(updates).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.AttachPrescriptionResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) DetachPrescription(
	ctx context.Context,
	req *connect.Request[posifacev1.DetachPrescriptionRequest],
) (*connect.Response[posifacev1.DetachPrescriptionResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		// Block detach if any current line still needs the Rx.
		var items []model.SaleItem
		if err := tx.Where("sale_id = ?", sale.ID).Find(&items).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		for _, it := range items {
			var med model.Medicine
			if err := tx.Where("id = ?", it.MedicineID).First(&med).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			if med.PrescriptionRequired {
				return connect.NewError(connect.CodeFailedPrecondition,
					errors.New("cart still contains Rx-required items; remove them before detaching"))
			}
		}
		return tx.Model(sale).Update("prescription_id", nil).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.DetachPrescriptionResponse{Sale: saleToProto(sale)}), nil
}

// ---------- Complete (FEFO + stock movements + sale numbering) ----------

func (s *Sales) CompleteSale(
	ctx context.Context,
	req *connect.Request[posifacev1.CompleteSaleRequest],
) (*connect.Response[posifacev1.CompleteSaleResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	paymentStr, err := paymentSourceToString(req.Msg.PaymentSource)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}

		var items []model.SaleItem
		if err := tx.Where("sale_id = ?", sale.ID).Order("created_at").Find(&items).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if len(items) == 0 {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("cart is empty"))
		}

		// Cash requires paid_amount >= total. BPJS/Other accept paid_amount=0.
		if paymentStr == paymentCash && req.Msg.PaidAmount < sale.Total {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("paid_amount less than total"))
		}

		now := time.Now()

		// For each item: pick FEFO batches with available qty, allocate.
		// If a line spans multiple batches, we split it into one item row per
		// batch consumed so audit is clean (1 sale_item -> 1 stock_movement).
		for _, item := range items {
			needed := int32(item.Qty)

			// Available batches for the medicine, ordered by expiry ASC (FEFO).
			// Compute per-batch available qty (sum of stock_movements) inside the loop.
			var batches []model.Batch
			if err := tx.Where("medicine_id = ?", item.MedicineID).
				Order("expiry_date ASC").
				Find(&batches).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			// Strip the placeholder item; we'll re-insert allocated children.
			if err := tx.Delete(&item).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			for _, b := range batches {
				if needed <= 0 {
					break
				}
				var avail int64
				if err := tx.Model(&model.StockMovement{}).
					Where("batch_id = ?", b.ID).
					Select("COALESCE(SUM(qty), 0)").
					Scan(&avail).Error; err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}
				if avail <= 0 {
					continue
				}
				take := int64(needed)
				if take > avail {
					take = avail
				}

				// Insert allocated child sale_item row pinned to this batch.
				batchID := b.ID
				child := model.SaleItem{
					SaleID:            sale.ID,
					MedicineID:        item.MedicineID,
					BatchID:           &batchID,
					Qty:               int32(take),
					UnitPriceSnapshot: item.UnitPriceSnapshot,
					LineDiscount:      0, // line discounts pre-FEFO would be reapportioned; v1 ignores
					LineTotal:         computeLineTotal(int32(take), item.UnitPriceSnapshot, 0),
				}
				if err := tx.Create(&child).Error; err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}

				// Insert the SALE stock movement (negative qty), linked to this sale_item.
				saleItemID := child.ID
				mv := model.StockMovement{
					BatchID:    b.ID,
					Qty:        -int32(take),
					Type:       movementTypeSale,
					Reason:     "POS sale",
					UserID:     caller.UserID,
					SaleItemID: &saleItemID,
				}
				if err := tx.Create(&mv).Error; err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}

				needed -= int32(take)
			}

			if needed > 0 {
				// Insufficient stock — abort the whole tx. Caller can adjust qty
				// or restock. Wrapped as FailedPrecondition.
				return connect.NewError(connect.CodeFailedPrecondition,
					fmt.Errorf("insufficient stock for medicine %s (%d remaining short)", item.MedicineID, needed))
			}
		}

		// Recompute totals from the now-allocated sale_items.
		if err := recomputeSaleTotals(tx, sale.ID); err != nil {
			return err
		}

		// Increment prescription dispensed_qty for any Rx-required medicines.
		if sale.PrescriptionID != nil && *sale.PrescriptionID != "" {
			if err := incrementRxDispensed(tx, *sale.PrescriptionID, items); err != nil {
				return err
			}
		}

		// Assign per-year sale_no.
		saleNo, err := assignSaleNo(tx, now)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Finalize the sale row.
		updates := map[string]any{
			"sale_no":        saleNo,
			"payment_source": paymentStr,
			"paid_amount":    req.Msg.PaidAmount,
			"status":         saleStatusCompleted,
			"completed_at":   now,
		}
		if err := tx.Model(sale).Updates(updates).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		sale.Status = saleStatusCompleted
		sale.CompletedAt = &now
		sale.SaleNo = &saleNo
		sale.PaymentSource = &paymentStr
		sale.PaidAmount = req.Msg.PaidAmount

		// If the customer is PKP (has NPWP), assign a tax-invoice number from
		// the NSFP pool. If the pool is empty the whole CompleteSale rolls back
		// — better to fail loudly than silently miss a tax invoice.
		if sale.CustomerID != nil && *sale.CustomerID != "" {
			var cust model.Customer
			if err := tx.Where("id = ?", *sale.CustomerID).First(&cust).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			if strings.TrimSpace(cust.NPWP) != "" {
				if err := AssignTaxInvoiceForSaleTx(tx, sale); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.CompleteSaleResponse{Sale: saleToProto(sale)}), nil
}

func (s *Sales) VoidSale(
	ctx context.Context,
	req *connect.Request[posifacev1.VoidSaleRequest],
) (*connect.Response[posifacev1.VoidSaleResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sale model.Sale
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", req.Msg.SaleId).First(&sale).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if sale.Status != saleStatusDraft {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only draft sales can be voided; this one is %s", sale.Status))
		}
		return tx.Model(&sale).Update("status", saleStatusVoided).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.VoidSaleResponse{Sale: saleToProto(sale)}), nil
}

// ---------- Snapshot ----------

func (s *Sales) GetTodaySnapshot(
	ctx context.Context,
	_ *connect.Request[posifacev1.GetTodaySnapshotRequest],
) (*connect.Response[posifacev1.GetTodaySnapshotResponse], error) {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var revenue int64
	if err := s.db.WithContext(ctx).Model(&model.Sale{}).
		Where("status = ? AND completed_at >= ?", saleStatusCompleted, dayStart).
		Select("COALESCE(SUM(total), 0)").
		Scan(&revenue).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var saleCount int64
	if err := s.db.WithContext(ctx).Model(&model.Sale{}).
		Where("status = ? AND completed_at >= ?", saleStatusCompleted, dayStart).
		Count(&saleCount).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var itemsSold int64
	if err := s.db.WithContext(ctx).
		Table("sale_items si").
		Joins("JOIN sales s ON s.id = si.sale_id").
		Where("s.status = ? AND s.completed_at >= ?", saleStatusCompleted, dayStart).
		Select("COALESCE(SUM(si.qty), 0)").
		Scan(&itemsSold).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type topRow struct {
		MedicineID string
		Qty        int64
	}
	var top topRow
	_ = s.db.WithContext(ctx).
		Table("sale_items si").
		Joins("JOIN sales s ON s.id = si.sale_id").
		Where("s.status = ? AND s.completed_at >= ?", saleStatusCompleted, dayStart).
		Select("si.medicine_id AS medicine_id, SUM(si.qty) AS qty").
		Group("si.medicine_id").
		Order("qty DESC").
		Limit(1).
		Scan(&top).Error

	return connect.NewResponse(&posifacev1.GetTodaySnapshotResponse{
		Revenue:        revenue,
		SaleCount:      saleCount,
		ItemsSold:      itemsSold,
		TopMedicineId:  top.MedicineID,
		TopMedicineQty: top.Qty,
	}), nil
}

// ---------- Print ----------

func (s *Sales) PrintReceipt(
	ctx context.Context,
	req *connect.Request[posifacev1.PrintReceiptRequest],
) (*connect.Response[posifacev1.PrintReceiptResponse], error) {
	if !s.printer.Enabled {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("printer is not configured (set printer.enabled in config.yaml)"))
	}
	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	if sale.Status != saleStatusCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("only completed sales can be printed"))
	}

	// Resolve medicine names.
	medIDs := make([]string, 0, len(sale.Items))
	for _, it := range sale.Items {
		medIDs = append(medIDs, it.MedicineID)
	}
	var meds []model.Medicine
	if err := s.db.WithContext(ctx).Where("id IN ?", medIDs).Find(&meds).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	medName := make(map[string]string, len(meds))
	for _, m := range meds {
		medName[m.ID] = m.Name
	}

	// Resolve cashier name.
	var cashier model.User
	cashierName := ""
	if err := s.db.WithContext(ctx).Where("id = ?", sale.CashierUserID).First(&cashier).Error; err == nil {
		cashierName = cashier.Name
		if cashierName == "" {
			cashierName = cashier.Email
		}
	}

	// Resolve customer name (optional).
	customerName := ""
	if sale.CustomerID != nil && *sale.CustomerID != "" {
		var customer model.Customer
		if err := s.db.WithContext(ctx).Where("id = ?", *sale.CustomerID).First(&customer).Error; err == nil {
			customerName = customer.Name
		}
	}

	// Build the renderer Receipt.
	lines := make([]printer.ReceiptLine, 0, len(sale.Items))
	for _, it := range sale.Items {
		lines = append(lines, printer.ReceiptLine{
			Qty:       it.Qty,
			Name:      medName[it.MedicineID],
			LineTotal: it.LineTotal,
		})
	}
	completedAt := time.Time{}
	if sale.CompletedAt != nil {
		completedAt = *sale.CompletedAt
	}
	saleNo := ""
	if sale.SaleNo != nil {
		saleNo = *sale.SaleNo
	}
	paymentStr := ""
	if sale.PaymentSource != nil {
		paymentStr = *sale.PaymentSource
	}
	change := int64(0)
	if paymentStr == paymentCash && sale.PaidAmount > sale.Total {
		change = sale.PaidAmount - sale.Total
	}

	receipt := printer.Receipt{
		SaleNo:      saleNo,
		CompletedAt: completedAt,
		Cashier:     cashierName,
		Customer:    customerName,
		Items:       lines,
		Subtotal:    sale.Subtotal,
		Total:       sale.Total,
		Paid:        sale.PaidAmount,
		Payment:     paymentStr,
		Change:      change,
	}
	settings := printer.Settings{
		Width:      s.printer.Width,
		Header:     s.printer.Header,
		Footer:     s.printer.Footer,
		OpenDrawer: s.printer.OpenDrawer,
	}
	payload := printer.Render(receipt, settings)

	if err := printer.DispatchTCP(s.printer.Address, payload, s.printer.Timeout); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&posifacev1.PrintReceiptResponse{
		BytesSent: int32(len(payload)),
	}), nil
}

// ---------- Helpers ----------

func (s *Sales) draftForUpdate(tx *gorm.DB, id string) (*model.Sale, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sale_id required"))
	}
	var sale model.Sale
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&sale).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if sale.Status != saleStatusDraft {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("sale is %s; only DRAFT sales accept mutations", sale.Status))
	}
	return &sale, nil
}

func (s *Sales) loadFull(ctx context.Context, id string) (*model.Sale, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sale_id required"))
	}
	var sale model.Sale
	err := s.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&sale).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &sale, nil
}

func recomputeSaleTotals(tx *gorm.DB, saleID string) error {
	var subtotal int64
	if err := tx.Model(&model.SaleItem{}).
		Where("sale_id = ?", saleID).
		Select("COALESCE(SUM(line_total), 0)").
		Scan(&subtotal).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	var current model.Sale
	if err := tx.Where("id = ?", saleID).First(&current).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	total := subtotal - current.CartDiscount
	if total < 0 {
		total = 0
	}
	return tx.Model(&current).Updates(map[string]any{
		"subtotal": subtotal,
		"total":    total,
	}).Error
}

func computeLineTotal(qty int32, unitPrice, lineDiscount int64) int64 {
	gross := int64(qty) * unitPrice
	net := gross - lineDiscount
	if net < 0 {
		return 0
	}
	return net
}

func assignSaleNo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	// Upsert + atomic increment.
	var counter model.SaleNoCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.SaleNoCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}

	// Atomic increment using SQL expression to keep concurrent CompleteSale
	// calls correct under row-lock.
	if err := tx.Model(&model.SaleNoCounter{}).
		Where("year = ?", year).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("INV-%d-%04d", year, counter.LastSeq), nil
}

// assertRxCovers checks that the sale has an active prescription covering
// `needQty` of `medicineID` (taking already-dispensed qty into account).
func assertRxCovers(tx *gorm.DB, sale *model.Sale, medicineID string, needQty int32) error {
	if sale.PrescriptionID == nil || *sale.PrescriptionID == "" {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("medicine requires a prescription; none attached to this sale"))
	}
	var rx model.Prescription
	if err := tx.Preload("Items").Where("id = ?", *sale.PrescriptionID).First(&rx).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if status := computeRxStatus(&rx, time.Now()); status != rxStatusActive {
		return connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("attached prescription is %s", status))
	}
	for _, it := range rx.Items {
		if it.MedicineID != medicineID {
			continue
		}
		remaining := it.PrescribedQty - it.DispensedQty
		if needQty > remaining {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("prescription remaining qty %d < requested %d for this medicine",
					remaining, needQty))
		}
		return nil
	}
	return connect.NewError(connect.CodeFailedPrecondition,
		errors.New("prescription does not include this medicine"))
}

// incrementRxDispensed bumps prescription_items.dispensed_qty for each sale
// item whose medicine is Rx-required and present on the prescription. Caller
// owns the tx; cart-line allocation has already succeeded.
func incrementRxDispensed(tx *gorm.DB, prescriptionID string, items []model.SaleItem) error {
	// Sum qty per medicine across the sale (FEFO may have split lines).
	totals := map[string]int32{}
	for _, it := range items {
		// it.MedicineID is the original cart-line medicine.
		totals[it.MedicineID] += it.Qty
	}
	for medID, qty := range totals {
		var med model.Medicine
		if err := tx.Where("id = ?", medID).First(&med).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if !med.PrescriptionRequired {
			continue
		}
		res := tx.Model(&model.PrescriptionItem{}).
			Where("prescription_id = ? AND medicine_id = ?", prescriptionID, medID).
			Update("dispensed_qty", gorm.Expr("dispensed_qty + ?", qty))
		if res.Error != nil {
			return connect.NewError(connect.CodeInternal, res.Error)
		}
		if res.RowsAffected == 0 {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("prescription does not cover medicine %s", medID))
		}
	}
	return nil
}

// asConnectErr passes through connect errors and wraps others as Internal.
func asConnectErr(err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeInternal, err)
}

// ---------- Proto mapping ----------

func saleToProto(s *model.Sale) *posifacev1.Sale {
	out := &posifacev1.Sale{
		Id:             s.ID,
		CashierUserId:  s.CashierUserID,
		Subtotal:       s.Subtotal,
		CartDiscount:   s.CartDiscount,
		Total:          s.Total,
		PaidAmount:     s.PaidAmount,
		Status:         saleStatusToProto(s.Status),
		CreatedAt:      s.CreatedAt.Unix(),
		PaymentSource:  paymentSourceFromString(deref(s.PaymentSource)),
	}
	if s.SaleNo != nil {
		out.SaleNo = *s.SaleNo
	}
	if s.CustomerID != nil {
		out.CustomerId = *s.CustomerID
	}
	if s.TaxInvoiceNo != nil {
		out.TaxInvoiceNo = *s.TaxInvoiceNo
	}
	if s.BranchID != nil {
		out.BranchId = *s.BranchID
	}
	if s.PrescriptionID != nil {
		out.PrescriptionId = *s.PrescriptionID
	}
	if s.CompletedAt != nil {
		out.CompletedAt = s.CompletedAt.Unix()
	}
	for i := range s.Items {
		out.Items = append(out.Items, saleItemToProto(&s.Items[i]))
	}
	return out
}

func saleItemToProto(i *model.SaleItem) *posifacev1.SaleItem {
	out := &posifacev1.SaleItem{
		Id:                i.ID,
		SaleId:            i.SaleID,
		MedicineId:        i.MedicineID,
		Qty:               i.Qty,
		UnitPriceSnapshot: i.UnitPriceSnapshot,
		LineDiscount:      i.LineDiscount,
		LineTotal:         i.LineTotal,
	}
	if i.BatchID != nil {
		out.BatchId = *i.BatchID
	}
	return out
}

func saleStatusToString(s posifacev1.SaleStatus) string {
	switch s {
	case posifacev1.SaleStatus_SALE_STATUS_DRAFT:
		return saleStatusDraft
	case posifacev1.SaleStatus_SALE_STATUS_COMPLETED:
		return saleStatusCompleted
	case posifacev1.SaleStatus_SALE_STATUS_VOIDED:
		return saleStatusVoided
	default:
		return ""
	}
}

func saleStatusToProto(s string) posifacev1.SaleStatus {
	switch s {
	case saleStatusDraft:
		return posifacev1.SaleStatus_SALE_STATUS_DRAFT
	case saleStatusCompleted:
		return posifacev1.SaleStatus_SALE_STATUS_COMPLETED
	case saleStatusVoided:
		return posifacev1.SaleStatus_SALE_STATUS_VOIDED
	default:
		return posifacev1.SaleStatus_SALE_STATUS_UNSPECIFIED
	}
}

func paymentSourceToString(p posifacev1.PaymentSource) (string, error) {
	switch p {
	case posifacev1.PaymentSource_PAYMENT_SOURCE_CASH:
		return paymentCash, nil
	case posifacev1.PaymentSource_PAYMENT_SOURCE_BPJS:
		return paymentBPJS, nil
	case posifacev1.PaymentSource_PAYMENT_SOURCE_INSURANCE_OTHER:
		return paymentInsuranceOther, nil
	default:
		return "", errors.New("payment_source required")
	}
}

func paymentSourceFromString(s string) posifacev1.PaymentSource {
	switch s {
	case paymentCash:
		return posifacev1.PaymentSource_PAYMENT_SOURCE_CASH
	case paymentBPJS:
		return posifacev1.PaymentSource_PAYMENT_SOURCE_BPJS
	case paymentInsuranceOther:
		return posifacev1.PaymentSource_PAYMENT_SOURCE_INSURANCE_OTHER
	default:
		return posifacev1.PaymentSource_PAYMENT_SOURCE_UNSPECIFIED
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
