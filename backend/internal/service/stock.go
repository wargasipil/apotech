package service

import (
	"context"
	"errors"
	"fmt"

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
	q := s.db.WithContext(ctx).Order("created_at DESC")
	if req.Msg.BatchId != "" {
		q = q.Where("batch_id = ?", req.Msg.BatchId)
	}
	if t := movementTypeToString(req.Msg.Type); t != "" {
		q = q.Where("type = ?", t)
	}

	var rows []model.StockMovement
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.StockMovement, 0, len(rows))
	for _, r := range rows {
		out = append(out, movementToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.ListMovementsResponse{Movements: out}), nil
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

	mv := model.StockMovement{
		BatchID: req.Msg.BatchId,
		Qty:     req.Msg.Qty,
		Type:    typeStr,
		Reason:  req.Msg.Reason,
		UserID:  caller.UserID,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&mv).Error; err != nil {
			return fmt.Errorf("create movement: %w", err)
		}
		// Guard: refuse if this movement would drive stock negative.
		qty, err := batchCurrentQty(ctx, tx, mv.BatchID)
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
	q := s.db.WithContext(ctx).
		Table("batches b").
		Select(`b.id AS batch_id,
		        b.medicine_id,
		        TO_CHAR(b.expiry_date, 'YYYY-MM-DD') AS expiry_date,
		        COALESCE(SUM(m.qty), 0) AS current_quantity`).
		Joins("LEFT JOIN stock_movements m ON m.batch_id = b.id").
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
