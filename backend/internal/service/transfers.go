package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/apotech/backend/gen/warehouse_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
	"github.com/apotech/backend/internal/sqldialect"
)

const (
	movementTransferOut = "TRANSFER_OUT"
	movementTransferIn  = "TRANSFER_IN"
)

type Transfers struct {
	db *gorm.DB
}

func NewTransfers(db *gorm.DB) *Transfers { return &Transfers{db: db} }

func (s *Transfers) CreateTransfer(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.CreateTransferRequest],
) (*connect.Response[warehouseifacev1.CreateTransferResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	from := req.Msg.FromWarehouseId
	to := req.Msg.ToWarehouseId
	if from == "" || to == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("from and to warehouse required"))
	}
	if from == to {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source and destination must differ"))
	}
	if len(req.Msg.Lines) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one line required"))
	}

	var transferID string
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Both warehouses must exist + be active.
		for _, id := range []string{from, to} {
			var wh model.Warehouse
			if e := tx.Where("id = ? AND active", id).First(&wh).Error; e != nil {
				if errors.Is(e, gorm.ErrRecordNotFound) {
					return connect.NewError(connect.CodeFailedPrecondition, errors.New("warehouse not found or inactive"))
				}
				return connect.NewError(connect.CodeInternal, e)
			}
		}

		now := time.Now()
		no, e := assignTransferNo(tx, now)
		if e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		header := model.StockTransfer{
			TransferNo:      &no,
			FromWarehouseID: from,
			ToWarehouseID:   to,
			Note:            req.Msg.Note,
			CreatedBy:       caller.UserID,
		}
		if e := tx.Create(&header).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		transferID = header.ID

		// Lock the line batches FOR UPDATE (deterministic id order) so concurrent
		// drains of the same source lot serialize and can't both pass the
		// availability check and oversell the source warehouse.
		batchIDs := make([]string, 0, len(req.Msg.Lines))
		for _, line := range req.Msg.Lines {
			batchIDs = append(batchIDs, line.BatchId)
		}
		if e := lockBatchesByID(tx, batchIDs); e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}

		for _, line := range req.Msg.Lines {
			if line.Qty <= 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("qty must be > 0"))
			}
			avail, e := batchQtyInWarehouse(ctx, tx, line.BatchId, from)
			if e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
			if int64(line.Qty) > avail {
				return connect.NewError(connect.CodeFailedPrecondition,
					fmt.Errorf("insufficient stock for batch %s in source warehouse (have %d, need %d)",
						line.BatchId, avail, line.Qty))
			}
			out := model.StockMovement{
				BatchID:     line.BatchId,
				Qty:         -line.Qty,
				Type:        movementTransferOut,
				Reason:      fmt.Sprintf("Transfer %s", no),
				UserID:      caller.UserID,
				WarehouseID: from,
				TransferID:  &transferID,
			}
			in := model.StockMovement{
				BatchID:     line.BatchId,
				Qty:         line.Qty,
				Type:        movementTransferIn,
				Reason:      fmt.Sprintf("Transfer %s", no),
				UserID:      caller.UserID,
				WarehouseID: to,
				TransferID:  &transferID,
			}
			if e := tx.Create(&out).Error; e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
			if e := tx.Create(&in).Error; e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	out, err := s.loadTransfer(ctx, transferID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&warehouseifacev1.CreateTransferResponse{Transfer: out}), nil
}

func (s *Transfers) ListTransfers(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.ListTransfersRequest],
) (*connect.Response[warehouseifacev1.ListTransfersResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	// Scope to the active warehouse — transfers touching it (from OR to). An
	// explicit warehouse_id in the request overrides; otherwise resolve from the
	// X-Warehouse-Id header like every other inventory read.
	wh := req.Msg.WarehouseId
	if wh == "" {
		wh, err = resolveWarehouse(ctx, s.db, caller)
		if err != nil {
			return nil, err
		}
	}
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Where("from_warehouse_id = ? OR to_warehouse_id = ?", wh, wh)
		if query := strings.TrimSpace(req.Msg.Query); query != "" {
			pattern := "%" + query + "%"
			q = q.Where(sqldialect.ILikeAny("transfer_no", "note"), pattern, pattern)
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
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StockTransfer{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.StockTransfer
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StockTransfer{})).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*warehouseifacev1.StockTransfer, 0, len(rows))
	for i := range rows {
		hydrated, err := s.hydrateTransfer(ctx, &rows[i], false)
		if err != nil {
			return nil, err
		}
		out = append(out, hydrated)
	}
	return connect.NewResponse(&warehouseifacev1.ListTransfersResponse{
		Transfers: out,
		Total:     int32(total),
	}), nil
}

func (s *Transfers) GetTransfer(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.GetTransferRequest],
) (*connect.Response[warehouseifacev1.GetTransferResponse], error) {
	out, err := s.loadTransfer(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&warehouseifacev1.GetTransferResponse{Transfer: out}), nil
}

// ---------- helpers ----------

func (s *Transfers) loadTransfer(ctx context.Context, id string) (*warehouseifacev1.StockTransfer, error) {
	var row model.StockTransfer
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("transfer not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return s.hydrateTransfer(ctx, &row, true)
}

func (s *Transfers) hydrateTransfer(
	ctx context.Context,
	row *model.StockTransfer,
	withLines bool,
) (*warehouseifacev1.StockTransfer, error) {
	out := &warehouseifacev1.StockTransfer{
		Id:              row.ID,
		FromWarehouseId: row.FromWarehouseID,
		ToWarehouseId:   row.ToWarehouseID,
		Note:            row.Note,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt.Unix(),
	}
	if row.TransferNo != nil {
		out.TransferNo = *row.TransferNo
	}
	// Warehouse names.
	var whs []model.Warehouse
	if err := s.db.WithContext(ctx).Where("id IN ?", []string{row.FromWarehouseID, row.ToWarehouseID}).
		Find(&whs).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, w := range whs {
		if w.ID == row.FromWarehouseID {
			out.FromWarehouseName = w.Name
		}
		if w.ID == row.ToWarehouseID {
			out.ToWarehouseName = w.Name
		}
	}
	if !withLines {
		return out, nil
	}
	// Reconstruct lines from the TRANSFER_IN movements (one per batch moved).
	type lineRow struct {
		BatchID      string `gorm:"column:batch_id"`
		Qty          int32  `gorm:"column:qty"`
		MedicineID   string `gorm:"column:medicine_id"`
		MedicineName string `gorm:"column:medicine_name"`
		BatchNumber  string `gorm:"column:batch_number"`
		ExpiryDate   string `gorm:"column:expiry_date"`
	}
	var rows []lineRow
	err := s.db.WithContext(ctx).
		Table("stock_movements AS m").
		Select(`m.batch_id, m.qty,
		        b.medicine_id AS medicine_id,
		        med.name AS medicine_name,
		        b.batch_number AS batch_number,
		        TO_CHAR(b.expiry_date, 'YYYY-MM-DD') AS expiry_date`).
		Joins("JOIN batches AS b ON b.id = m.batch_id").
		Joins("JOIN medicines AS med ON med.id = b.medicine_id").
		Where("m.transfer_id = ? AND m.type = ?", row.ID, movementTransferIn).
		Order("med.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, r := range rows {
		out.Lines = append(out.Lines, &warehouseifacev1.StockTransferLine{
			BatchId:      r.BatchID,
			MedicineId:   r.MedicineID,
			MedicineName: r.MedicineName,
			BatchNumber:  r.BatchNumber,
			ExpiryDate:   r.ExpiryDate,
			Qty:          r.Qty,
		})
	}
	return out, nil
}

func assignTransferNo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	var counter model.TransferCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.TransferCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := tx.Model(&model.TransferCounter{}).
		Where("year = ?", year).
		Clauses(sqldialect.LockForUpdate()).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("TRF-%d-%04d", year, counter.LastSeq), nil
}
