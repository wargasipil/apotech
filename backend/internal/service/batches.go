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

const dateLayout = "2006-01-02"

type Batches struct {
	db *gorm.DB
}

func NewBatches(db *gorm.DB) *Batches { return &Batches{db: db} }

func (b *Batches) ListBatches(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListBatchesRequest],
) (*connect.Response[inventoryifacev1.ListBatchesResponse], error) {
	q := b.db.WithContext(ctx).Order("expiry_date ASC")
	if req.Msg.MedicineId != "" {
		q = q.Where("medicine_id = ?", req.Msg.MedicineId)
	}
	var rows []model.Batch
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.Batch, 0, len(rows))
	for _, r := range rows {
		qty, err := batchCurrentQty(ctx, b.db, r.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if req.Msg.OnlyInStock && qty <= 0 {
			continue
		}
		out = append(out, batchToProto(&r, qty))
	}
	return connect.NewResponse(&inventoryifacev1.ListBatchesResponse{Batches: out}), nil
}

func (b *Batches) GetBatch(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetBatchRequest],
) (*connect.Response[inventoryifacev1.GetBatchResponse], error) {
	batch, err := b.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	qty, err := batchCurrentQty(ctx, b.db, batch.ID)
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

	err = b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batch).Error; err != nil {
			return fmt.Errorf("create batch: %w", err)
		}
		if req.Msg.InitialQuantity > 0 {
			mv := model.StockMovement{
				BatchID: batch.ID,
				Qty:     int32(req.Msg.InitialQuantity),
				Type:    "PURCHASE",
				Reason:  "initial stock",
				UserID:  caller.UserID,
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

// batchCurrentQty returns SUM(stock_movements.qty) for a batch.
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
