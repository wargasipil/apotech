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

const dateLayout = "2006-01-02"

type Batches struct {
	db *gorm.DB
}

func NewBatches(db *gorm.DB) *Batches { return &Batches{db: db} }

func (b *Batches) ListBatches(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListBatchesRequest],
) (*connect.Response[inventoryifacev1.ListBatchesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, b.db, caller)
	if err != nil {
		return nil, err
	}

	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)

	// Per-warehouse stock is computed in SQL (GROUP BY) so only_in_stock +
	// pagination + total stay consistent.
	base := func() *gorm.DB {
		q := b.db.WithContext(ctx).
			Table("batches AS b").
			Joins("LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", warehouseID).
			Group("b.id")
		if req.Msg.MedicineId != "" {
			q = q.Where("b.medicine_id = ?", req.Msg.MedicineId)
		}
		if sid := strings.TrimSpace(req.Msg.SupplierId); sid != "" {
			q = q.Where("b.supplier_id = ?", sid)
		}
		if req.Msg.OnlyInStock {
			q = q.Having("COALESCE(SUM(sm.qty), 0) > 0")
		}
		if query := strings.TrimSpace(req.Msg.Query); query != "" {
			pattern := "%" + query + "%"
			sub := b.db.Table("batches AS b2").
				Select("b2.id").
				Joins("JOIN medicines m ON m.id = b2.medicine_id").
				Where("b2.batch_number ILIKE ? OR m.name ILIKE ? OR m.sku ILIKE ?", pattern, pattern, pattern)
			q = q.Where("b.id IN (?)", sub)
		}
		if req.Msg.FromUnix > 0 || req.Msg.ToUnix > 0 {
			col := "b.received_at"
			if req.Msg.DateField == "expiry" {
				col = "b.expiry_date"
			}
			if req.Msg.FromUnix > 0 {
				q = q.Where(col+" >= ?", time.Unix(req.Msg.FromUnix, 0))
			}
			if req.Msg.ToUnix > 0 {
				q = q.Where(col+" < ?", time.Unix(req.Msg.ToUnix, 0))
			}
		}
		return q
	}

	var total int64
	if err := b.db.WithContext(ctx).
		Table("(?) AS sub", base().Select("b.id")).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	type batchRow struct {
		model.Batch
		Qty int64 `gorm:"column:qty"`
	}
	var rows []batchRow
	if err := base().
		Select("b.*, COALESCE(SUM(sm.qty), 0) AS qty").
		Order("b.expiry_date ASC").Offset(offset).Limit(limit).Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Batch, 0, len(rows))
	for i := range rows {
		out = append(out, batchToProto(&rows[i].Batch, rows[i].Qty))
	}
	// Enrich each row with its originating PO (via the receipt that created
	// it). Empty for legacy / manual CreateBatch path. One batched query.
	if len(out) > 0 {
		batchIDs := make([]string, 0, len(out))
		for _, x := range out {
			batchIDs = append(batchIDs, x.Id)
		}
		type poRow struct {
			BatchID         string `gorm:"column:batch_id"`
			PurchaseOrderID string `gorm:"column:purchase_order_id"`
			PoNo            string `gorm:"column:po_no"`
		}
		var poRows []poRow
		if err := b.db.WithContext(ctx).
			Table("purchase_receipt_items AS pri").
			Select("pri.batch_id AS batch_id, po.id AS purchase_order_id, COALESCE(po.po_no, '') AS po_no").
			Joins("JOIN purchase_receipts pr ON pr.id = pri.purchase_receipt_id").
			Joins("JOIN purchase_orders po ON po.id = pr.purchase_order_id").
			Where("pri.batch_id IN ?", batchIDs).
			Scan(&poRows).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		byBatch := make(map[string]poRow, len(poRows))
		for _, r := range poRows {
			byBatch[r.BatchID] = r
		}
		for _, x := range out {
			if r, ok := byBatch[x.Id]; ok {
				x.PurchaseOrderId = r.PurchaseOrderID
				x.PoNo = r.PoNo
			}
		}
	}
	return connect.NewResponse(&inventoryifacev1.ListBatchesResponse{
		Batches: out,
		Total:   int32(total),
	}), nil
}

func (b *Batches) GetBatch(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetBatchRequest],
) (*connect.Response[inventoryifacev1.GetBatchResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, b.db, caller)
	if err != nil {
		return nil, err
	}
	batch, err := b.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	qty, err := batchQtyInWarehouse(ctx, b.db, batch.ID, warehouseID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.GetBatchResponse{Batch: batchToProto(batch, qty)}), nil
}

func (b *Batches) CreateBatch(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateBatchRequest],
) (*connect.Response[inventoryifacev1.CreateBatchResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	if req.Msg.MedicineId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("medicine_id required"))
	}
	expiry, err := time.Parse(dateLayout, req.Msg.ExpiryDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("expiry_date must be YYYY-MM-DD: %w", err))
	}
	received := time.Now()
	if req.Msg.ReceivedAt != "" {
		received, err = time.Parse(dateLayout, req.Msg.ReceivedAt)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("received_at must be YYYY-MM-DD: %w", err))
		}
	}
	if req.Msg.InitialQuantity < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("initial_quantity must be >= 0"))
	}
	if req.Msg.CostPrice < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cost_price must be >= 0"))
	}

	batch := model.Batch{
		MedicineID:  req.Msg.MedicineId,
		BatchNumber: strings.TrimSpace(req.Msg.BatchNumber),
		ExpiryDate:  expiry,
		CostPrice:   req.Msg.CostPrice,
		ReceivedAt:  received,
	}
	if req.Msg.SupplierId != "" {
		sid := req.Msg.SupplierId
		batch.SupplierID = &sid
	}

	warehouseID, err := resolveWarehouse(ctx, b.db, caller)
	if err != nil {
		return nil, err
	}

	err = b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batch).Error; err != nil {
			return fmt.Errorf("create batch: %w", err)
		}
		if req.Msg.InitialQuantity > 0 {
			mv := model.StockMovement{
				BatchID:     batch.ID,
				Qty:         int32(req.Msg.InitialQuantity),
				Type:        "PURCHASE",
				Reason:      "initial stock",
				UserID:      caller.UserID,
				WarehouseID: warehouseID,
			}
			if err := tx.Create(&mv).Error; err != nil {
				return fmt.Errorf("create initial movement: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	qty := int64(req.Msg.InitialQuantity)
	return connect.NewResponse(&inventoryifacev1.CreateBatchResponse{Batch: batchToProto(&batch, qty)}), nil
}

func (b *Batches) UpdateBatch(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateBatchRequest],
) (*connect.Response[inventoryifacev1.UpdateBatchResponse], error) {
	batch, err := b.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{
		"batch_number": strings.TrimSpace(req.Msg.BatchNumber),
		"cost_price":   req.Msg.CostPrice,
	}
	if req.Msg.SupplierId != "" {
		updates["supplier_id"] = req.Msg.SupplierId
	}
	if req.Msg.ExpiryDate != "" {
		expiry, err := time.Parse(dateLayout, req.Msg.ExpiryDate)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("expiry_date must be YYYY-MM-DD: %w", err))
		}
		updates["expiry_date"] = expiry
	}
	if req.Msg.ReceivedAt != "" {
		received, err := time.Parse(dateLayout, req.Msg.ReceivedAt)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("received_at must be YYYY-MM-DD: %w", err))
		}
		updates["received_at"] = received
	}

	if err := b.db.WithContext(ctx).Model(batch).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	batch, err = b.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	qty, err := batchCurrentQty(ctx, b.db, batch.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&inventoryifacev1.UpdateBatchResponse{Batch: batchToProto(batch, qty)}), nil
}

func (b *Batches) SearchBatches(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SearchBatchesRequest],
) (*connect.Response[inventoryifacev1.SearchBatchesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, b.db, caller)
	if err != nil {
		return nil, err
	}
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := b.db.WithContext(ctx).
		Table("batches AS b").
		Joins("JOIN medicines AS m ON m.id = b.medicine_id").
		Order("b.expiry_date ASC").
		Limit(limit).
		Select("b.*")
	if req.Msg.MedicineId != "" {
		q = q.Where("b.medicine_id = ?", req.Msg.MedicineId)
	}
	if query != "" {
		pattern := "%" + query + "%"
		q = q.Where("b.batch_number ILIKE ? OR m.name ILIKE ? OR m.sku ILIKE ?", pattern, pattern, pattern)
	}
	var rows []model.Batch
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Batch, 0, len(rows))
	for _, r := range rows {
		qty, err := batchQtyInWarehouse(ctx, b.db, r.ID, warehouseID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out = append(out, batchToProto(&r, qty))
	}
	return connect.NewResponse(&inventoryifacev1.SearchBatchesResponse{Batches: out}), nil
}

// ResolveBatches returns minimal display refs (batch number + medicine name) for
// a set of ids. Unknown ids are omitted; empty input returns an empty list.
func (b *Batches) ResolveBatches(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ResolveBatchesRequest],
) (*connect.Response[inventoryifacev1.ResolveBatchesResponse], error) {
	ids := dedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&inventoryifacev1.ResolveBatchesResponse{}), nil
	}
	type row struct {
		ID           string `gorm:"column:id"`
		BatchNumber  string `gorm:"column:batch_number"`
		MedicineID   string `gorm:"column:medicine_id"`
		MedicineName string `gorm:"column:medicine_name"`
	}
	var rows []row
	if err := b.db.WithContext(ctx).
		Table("batches AS bt").
		Select("bt.id, bt.batch_number, bt.medicine_id, m.name AS medicine_name").
		Joins("JOIN medicines m ON m.id = bt.medicine_id").
		Where("bt.id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.BatchRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.BatchRef{
			Id:           r.ID,
			BatchNumber:  r.BatchNumber,
			MedicineId:   r.MedicineID,
			MedicineName: r.MedicineName,
		})
	}
	return connect.NewResponse(&inventoryifacev1.ResolveBatchesResponse{Batches: out}), nil
}

func (b *Batches) load(ctx context.Context, id string) (*model.Batch, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var batch model.Batch
	err := b.db.WithContext(ctx).Where("id = ?", id).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("batch %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &batch, nil
}

// batchCurrentQty returns SUM(stock_movements.qty) for a batch across ALL
// warehouses (the global lot total). Used where location is irrelevant.
func batchCurrentQty(ctx context.Context, db *gorm.DB, batchID string) (int64, error) {
	var total *int64
	err := db.WithContext(ctx).
		Model(&model.StockMovement{}).
		Where("batch_id = ?", batchID).
		Select("COALESCE(SUM(qty), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

// lockBatchesByID takes a FOR UPDATE lock on the given batch rows in a
// deterministic order (by id) so concurrent stock-mutating txs serialize per lot
// without deadlocking (classic ordered locking). Call inside a tx before
// reading/consuming a batch's stock; the batches row acts as the per-lot mutex
// over the insert-only stock_movements ledger. No-op on empty input.
func lockBatchesByID(tx *gorm.DB, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	var dump []model.Batch
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).
		Order("id").
		Find(&dump).Error
}

// lockBatchesByMedicine FOR UPDATE-locks every batch (lot) of the given medicines
// in deterministic id order. Used by CompleteSale to serialize FEFO consumption
// of a medicine's lots across concurrent sales. No-op on empty input.
func lockBatchesByMedicine(tx *gorm.DB, medicineIDs []string) error {
	if len(medicineIDs) == 0 {
		return nil
	}
	var dump []model.Batch
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("medicine_id IN ?", medicineIDs).
		Order("id").
		Find(&dump).Error
}

// batchQtyInWarehouse returns SUM(qty) for a batch within one warehouse.
// This is the per-location stock figure that POS FEFO and transfers consume.
func batchQtyInWarehouse(ctx context.Context, db *gorm.DB, batchID, warehouseID string) (int64, error) {
	var total *int64
	err := db.WithContext(ctx).
		Model(&model.StockMovement{}).
		Where("batch_id = ? AND warehouse_id = ?", batchID, warehouseID).
		Select("COALESCE(SUM(qty), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

func batchToProto(b *model.Batch, qty int64) *inventoryifacev1.Batch {
	out := &inventoryifacev1.Batch{
		Id:              b.ID,
		MedicineId:      b.MedicineID,
		BatchNumber:     b.BatchNumber,
		ExpiryDate:      b.ExpiryDate.Format(dateLayout),
		CostPrice:       b.CostPrice,
		ReceivedAt:      b.ReceivedAt.Format(dateLayout),
		CurrentQuantity: qty,
		CreatedAt:       b.CreatedAt.Unix(),
	}
	if b.SupplierID != nil {
		out.SupplierId = *b.SupplierID
	}
	return out
}
