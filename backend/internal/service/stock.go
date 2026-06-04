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

type Stock struct {
	db *gorm.DB
}

func NewStock(db *gorm.DB) *Stock { return &Stock{db: db} }

func (s *Stock) ListMovements(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListMovementsRequest],
) (*connect.Response[inventoryifacev1.ListMovementsResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		// Scope to the caller's active warehouse so the Mutasi page + the
		// medicine-detail Movements tab + the Mutasi CSV export all show only
		// this warehouse's ledger.
		q = q.Where("warehouse_id = ?", warehouseID)
		if req.Msg.BatchId != "" {
			q = q.Where("batch_id = ?", req.Msg.BatchId)
		}
		if req.Msg.MedicineId != "" {
			q = q.Where("batch_id IN (?)",
				s.db.Table("batches").Select("id").Where("medicine_id = ?", req.Msg.MedicineId))
		}
		if t := movementTypeToString(req.Msg.Type); t != "" {
			q = q.Where("type = ?", t)
		}
		if query := strings.TrimSpace(req.Msg.Query); query != "" {
			pattern := "%" + query + "%"
			q = q.Where("batch_id IN (?)",
				s.db.Table("batches b").
					Select("b.id").
					Joins("JOIN medicines m ON m.id = b.medicine_id").
					Where(fmt.Sprintf("b.batch_number %[1]s ? OR m.name %[1]s ? OR m.sku %[1]s ?", likeKeyword(s.db)), pattern, pattern, pattern))
		}
		if req.Msg.FromUnix > 0 {
			q = q.Where("created_at >= ?", time.Unix(req.Msg.FromUnix, 0))
		}
		if req.Msg.ToUnix > 0 {
			q = q.Where("created_at < ?", time.Unix(req.Msg.ToUnix, 0))
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StockMovement{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.StockMovement
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StockMovement{})).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.StockMovement, 0, len(rows))
	for _, r := range rows {
		out = append(out, movementToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.ListMovementsResponse{
		Movements: out,
		Total:     int32(total),
	}), nil
}

func (s *Stock) RecordMovement(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.RecordMovementRequest],
) (*connect.Response[inventoryifacev1.RecordMovementResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	if req.Msg.BatchId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("batch_id required"))
	}
	if req.Msg.Qty == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("qty must not be zero"))
	}

	// Restrict allowed types for this RPC. PURCHASE comes via CreateBatch;
	// SALE will come from POS in a later phase.
	var typeStr string
	switch req.Msg.Type {
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT:
		typeStr = "ADJUSTMENT"
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_WRITE_OFF:
		typeStr = "WRITE_OFF"
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("type must be ADJUSTMENT or WRITE_OFF for RecordMovement"))
	}

	warehouseID, err := resolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}

	mv := model.StockMovement{
		BatchID:     req.Msg.BatchId,
		Qty:         req.Msg.Qty,
		Type:        typeStr,
		Reason:      req.Msg.Reason,
		UserID:      caller.UserID,
		WarehouseID: warehouseID,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the batch lot FOR UPDATE first so the post-insert negative guard is
		// reliable against a concurrent sale/transfer of the same lot.
		if err := lockBatchesByID(tx, []string{mv.BatchID}); err != nil {
			return err
		}
		if err := tx.Create(&mv).Error; err != nil {
			return fmt.Errorf("create movement: %w", err)
		}
		// Guard: refuse if this movement would drive the batch's stock in this
		// warehouse negative.
		qty, err := batchQtyInWarehouse(ctx, tx, mv.BatchID, warehouseID)
		if err != nil {
			return err
		}
		if qty < 0 {
			return fmt.Errorf("movement would drive stock negative (current=%d)", qty)
		}
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	return connect.NewResponse(&inventoryifacev1.RecordMovementResponse{Movement: movementToProto(&mv)}), nil
}

func (s *Stock) GetStockLevels(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetStockLevelsRequest],
) (*connect.Response[inventoryifacev1.GetStockLevelsResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}

	// Stock is scoped to the caller's active warehouse — the warehouse filter
	// lives in the JOIN so batches with no stock there still appear (qty 0).
	q := s.db.WithContext(ctx).
		Table("batches b").
		Select(fmt.Sprintf(`b.id AS batch_id,
		        b.medicine_id,
		        %s AS expiry_date,
		        COALESCE(SUM(m.qty), 0) AS current_quantity`, dateText(s.db, "b.expiry_date"))).
		Joins("LEFT JOIN stock_movements m ON m.batch_id = b.id AND m.warehouse_id = ?", warehouseID).
		Group("b.id").
		Order("b.expiry_date ASC")

	if req.Msg.MedicineId != "" {
		q = q.Where("b.medicine_id = ?", req.Msg.MedicineId)
	}

	type row struct {
		BatchID         string `gorm:"column:batch_id"`
		MedicineID      string `gorm:"column:medicine_id"`
		ExpiryDate      string `gorm:"column:expiry_date"`
		CurrentQuantity int64  `gorm:"column:current_quantity"`
	}
	var rows []row
	if err := q.Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.StockLevel, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.StockLevel{
			BatchId:         r.BatchID,
			MedicineId:      r.MedicineID,
			ExpiryDate:      r.ExpiryDate,
			CurrentQuantity: r.CurrentQuantity,
		})
	}
	return connect.NewResponse(&inventoryifacev1.GetStockLevelsResponse{Levels: out}), nil
}

func movementToProto(m *model.StockMovement) *inventoryifacev1.StockMovement {
	return &inventoryifacev1.StockMovement{
		Id:        m.ID,
		BatchId:   m.BatchID,
		Qty:       m.Qty,
		Type:      movementTypeFromString(m.Type),
		Reason:    m.Reason,
		UserId:    m.UserID,
		CreatedAt: m.CreatedAt.Unix(),
	}
}

func movementTypeToString(t inventoryifacev1.MovementType) string {
	switch t {
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_PURCHASE:
		return "PURCHASE"
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_SALE:
		return "SALE"
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT:
		return "ADJUSTMENT"
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_WRITE_OFF:
		return "WRITE_OFF"
	default:
		return ""
	}
}

func movementTypeFromString(s string) inventoryifacev1.MovementType {
	switch s {
	case "PURCHASE":
		return inventoryifacev1.MovementType_MOVEMENT_TYPE_PURCHASE
	case "SALE":
		return inventoryifacev1.MovementType_MOVEMENT_TYPE_SALE
	case "ADJUSTMENT":
		return inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT
	case "WRITE_OFF":
		return inventoryifacev1.MovementType_MOVEMENT_TYPE_WRITE_OFF
	default:
		return inventoryifacev1.MovementType_MOVEMENT_TYPE_UNSPECIFIED
	}
}
