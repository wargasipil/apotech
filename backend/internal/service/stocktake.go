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

	stocktakeifacev1 "github.com/apotech/backend/gen/stocktake_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
)

const (
	stocktakeStatusDraft     = "DRAFT"
	stocktakeStatusCompleted = "COMPLETED"
	stocktakeStatusVoided    = "VOIDED"

	dispositionAdjustment = "ADJUSTMENT"
	dispositionWriteOff   = "WRITE_OFF"
)

var validWriteOffKinds = map[string]bool{
	"EXPIRED": true,
	"DAMAGED": true,
	"LOST":    true,
	"THEFT":   true,
	"OTHER":   true,
}

type Stocktakes struct {
	db *gorm.DB
}

func NewStocktakes(db *gorm.DB) *Stocktakes { return &Stocktakes{db: db} }

func (s *Stocktakes) StartStocktake(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.StartStocktakeRequest],
) (*connect.Response[stocktakeifacev1.StartStocktakeResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	// One DRAFT per warehouse — reject only if a draft is already open in THIS
	// warehouse (counting two warehouses concurrently is allowed).
	var existing int64
	if err := s.db.WithContext(ctx).Model(&model.StocktakeSession{}).
		Where("status = ? AND warehouse_id = ?", stocktakeStatusDraft, warehouseID).
		Count(&existing).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if existing > 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("a draft stocktake is already open in this warehouse"))
	}

	session := model.StocktakeSession{
		Name:        strings.TrimSpace(req.Msg.Name),
		Status:      stocktakeStatusDraft,
		CreatedBy:   caller.UserID,
		WarehouseID: &warehouseID,
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out, err := s.hydrateSession(ctx, &session)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.StartStocktakeResponse{Session: out}), nil
}

func (s *Stocktakes) AddBatchesToSession(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.AddBatchesToSessionRequest],
) (*connect.Response[stocktakeifacev1.AddBatchesToSessionResponse], error) {
	added, skipped, err := s.addBatches(ctx, req.Msg.SessionId, req.Msg.BatchIds)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.AddBatchesToSessionResponse{
		AddedCount:   added,
		SkippedCount: skipped,
	}), nil
}

func (s *Stocktakes) AddAllInStockBatches(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.AddAllInStockBatchesRequest],
) (*connect.Response[stocktakeifacev1.AddAllInStockBatchesResponse], error) {
	// Resolve the session's warehouse so we only seed batches in stock THERE.
	var session model.StocktakeSession
	if err := s.db.WithContext(ctx).Where("id = ?", req.Msg.SessionId).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("stocktake not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	whID := deref(session.WarehouseID)

	// Find every batch with current qty > 0 in this warehouse.
	type batchRow struct {
		BatchID string `gorm:"column:batch_id"`
	}
	var rows []batchRow
	err := s.db.WithContext(ctx).
		Table("batches b").
		Select("b.id AS batch_id").
		Joins("LEFT JOIN stock_movements m ON m.batch_id = b.id AND m.warehouse_id = ?", whID).
		Group("b.id").
		Having("COALESCE(SUM(m.qty), 0) > 0").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.BatchID)
	}
	added, skipped, err := s.addBatches(ctx, req.Msg.SessionId, ids)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.AddAllInStockBatchesResponse{
		AddedCount:   added,
		SkippedCount: skipped,
	}), nil
}

// addBatches snapshots expected_qty for each batch and inserts lines.
// Existing (session_id, batch_id) pairs are silently skipped (unique
// constraint + ON CONFLICT DO NOTHING).
func (s *Stocktakes) addBatches(
	ctx context.Context,
	sessionID string,
	batchIDs []string,
) (added int32, skipped int32, err error) {
	if sessionID == "" {
		return 0, 0, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id required"))
	}
	if len(batchIDs) == 0 {
		return 0, 0, nil
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sess, ierr := lockDraftSession(tx, sessionID)
		if ierr != nil {
			return ierr
		}
		whID := deref(sess.WarehouseID)
		for _, bid := range batchIDs {
			// expected_qty is snapshotted per the session's warehouse.
			qty, ierr := batchQtyInWarehouse(ctx, tx, bid, whID)
			if ierr != nil {
				return connect.NewError(connect.CodeInternal, ierr)
			}
			line := model.StocktakeLine{
				SessionID:   sessionID,
				BatchID:     bid,
				ExpectedQty: int32(qty),
				Disposition: dispositionAdjustment,
			}
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&line)
			if res.Error != nil {
				return connect.NewError(connect.CodeInternal, res.Error)
			}
			if res.RowsAffected > 0 {
				added++
			} else {
				skipped++
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, asConnectErr(err)
	}
	return added, skipped, nil
}

func (s *Stocktakes) RecordCount(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.RecordCountRequest],
) (*connect.Response[stocktakeifacev1.RecordCountResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.CountedQty < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("counted_qty must be >= 0"))
	}
	var line model.StocktakeLine
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if ierr := tx.Where("id = ?", req.Msg.LineId).First(&line).Error; ierr != nil {
			if errors.Is(ierr, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("line not found"))
			}
			return connect.NewError(connect.CodeInternal, ierr)
		}
		if _, ierr := lockDraftSession(tx, line.SessionID); ierr != nil {
			return ierr
		}
		now := time.Now()
		qty := req.Msg.CountedQty
		userID := caller.UserID
		return tx.Model(&line).Updates(map[string]any{
			"counted_qty": qty,
			"counted_at":  now,
			"counted_by":  userID,
		}).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	full, err := s.loadLine(ctx, req.Msg.LineId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.RecordCountResponse{Line: full}), nil
}

func (s *Stocktakes) SetLineDisposition(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.SetLineDispositionRequest],
) (*connect.Response[stocktakeifacev1.SetLineDispositionResponse], error) {
	disposition := strings.ToUpper(strings.TrimSpace(req.Msg.Disposition))
	if disposition != dispositionAdjustment && disposition != dispositionWriteOff {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("disposition must be ADJUSTMENT or WRITE_OFF"))
	}
	kind := strings.ToUpper(strings.TrimSpace(req.Msg.WriteOffKind))
	if disposition == dispositionWriteOff {
		if kind == "" || !validWriteOffKinds[kind] {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("write_off_kind must be EXPIRED|DAMAGED|LOST|THEFT|OTHER when disposition is WRITE_OFF"))
		}
	} else {
		kind = "" // strip kind for ADJUSTMENT lines
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var line model.StocktakeLine
		if ierr := tx.Where("id = ?", req.Msg.LineId).First(&line).Error; ierr != nil {
			if errors.Is(ierr, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("line not found"))
			}
			return connect.NewError(connect.CodeInternal, ierr)
		}
		if _, ierr := lockDraftSession(tx, line.SessionID); ierr != nil {
			return ierr
		}
		updates := map[string]any{
			"disposition":      disposition,
			"disposition_note": strings.TrimSpace(req.Msg.DispositionNote),
		}
		if kind == "" {
			updates["write_off_kind"] = nil
		} else {
			updates["write_off_kind"] = kind
		}
		return tx.Model(&line).Updates(updates).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	full, err := s.loadLine(ctx, req.Msg.LineId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.SetLineDispositionResponse{Line: full}), nil
}

func (s *Stocktakes) RemoveLine(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.RemoveLineRequest],
) (*connect.Response[stocktakeifacev1.RemoveLineResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var line model.StocktakeLine
		if ierr := tx.Where("id = ?", req.Msg.LineId).First(&line).Error; ierr != nil {
			if errors.Is(ierr, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("line not found"))
			}
			return connect.NewError(connect.CodeInternal, ierr)
		}
		if _, ierr := lockDraftSession(tx, line.SessionID); ierr != nil {
			return ierr
		}
		return tx.Delete(&line).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	return connect.NewResponse(&stocktakeifacev1.RemoveLineResponse{}), nil
}

func (s *Stocktakes) CompleteStocktake(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.CompleteStocktakeRequest],
) (*connect.Response[stocktakeifacev1.CompleteStocktakeResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var movementsWritten int32
	var session model.StocktakeSession
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sess, ierr := lockDraftSession(tx, req.Msg.SessionId)
		if ierr != nil {
			return ierr
		}
		session = *sess

		var lines []model.StocktakeLine
		if ierr := tx.Where("session_id = ?", session.ID).Find(&lines).Error; ierr != nil {
			return connect.NewError(connect.CodeInternal, ierr)
		}

		// Lock the lines' batch lots FOR UPDATE (deterministic id order) so the
		// per-line negative guard is reliable against a concurrent sale of the
		// same lot.
		seen := make(map[string]struct{}, len(lines))
		batchIDs := make([]string, 0, len(lines))
		for _, l := range lines {
			if _, ok := seen[l.BatchID]; ok {
				continue
			}
			seen[l.BatchID] = struct{}{}
			batchIDs = append(batchIDs, l.BatchID)
		}
		if ierr := lockBatchesByID(tx, batchIDs); ierr != nil {
			return connect.NewError(connect.CodeInternal, ierr)
		}

		// Validate every line before writing any movement.
		for _, l := range lines {
			if l.CountedQty == nil {
				continue
			}
			variance := *l.CountedQty - l.ExpectedQty
			if l.Disposition == dispositionWriteOff {
				if l.WriteOffKind == nil || *l.WriteOffKind == "" {
					return connect.NewError(connect.CodeFailedPrecondition,
						fmt.Errorf("line %s: WRITE_OFF disposition requires a write_off_kind", l.ID))
				}
				if variance > 0 {
					return connect.NewError(connect.CodeFailedPrecondition,
						fmt.Errorf("line %s: positive variance (%d) cannot be a WRITE_OFF", l.ID, variance))
				}
			}
		}

		// Write one movement per counted line with non-zero variance.
		for _, l := range lines {
			if l.CountedQty == nil {
				continue
			}
			variance := *l.CountedQty - l.ExpectedQty
			if variance == 0 {
				continue
			}
			lineID := l.ID
			reasonParts := []string{fmt.Sprintf("Stocktake: %s", session.Name)}
			if l.Disposition == dispositionWriteOff && l.WriteOffKind != nil {
				reasonParts = append(reasonParts, *l.WriteOffKind)
			}
			if l.DispositionNote != "" {
				reasonParts = append(reasonParts, l.DispositionNote)
			}
			mv := model.StockMovement{
				BatchID:         l.BatchID,
				Qty:             variance,
				Type:            l.Disposition,
				Reason:          strings.Join(reasonParts, " — "),
				UserID:          caller.UserID,
				WarehouseID:     deref(session.WarehouseID),
				StocktakeLineID: &lineID,
				WriteOffKind:    l.WriteOffKind,
			}
			if ierr := tx.Create(&mv).Error; ierr != nil {
				return connect.NewError(connect.CodeInternal, ierr)
			}
			// Guard: refuse if the resulting stock in this warehouse goes negative.
			qty, ierr := batchQtyInWarehouse(ctx, tx, l.BatchID, deref(session.WarehouseID))
			if ierr != nil {
				return connect.NewError(connect.CodeInternal, ierr)
			}
			if qty < 0 {
				return connect.NewError(connect.CodeFailedPrecondition,
					fmt.Errorf("line %s: applying variance would drive stock negative", l.ID))
			}
			movementsWritten++
		}

		now := time.Now()
		if ierr := tx.Model(&session).Updates(map[string]any{
			"status":       stocktakeStatusCompleted,
			"completed_at": now,
		}).Error; ierr != nil {
			return connect.NewError(connect.CodeInternal, ierr)
		}
		session.Status = stocktakeStatusCompleted
		session.CompletedAt = &now
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	out, err := s.hydrateSession(ctx, &session)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.CompleteStocktakeResponse{
		Session:          out,
		MovementsWritten: movementsWritten,
	}), nil
}

func (s *Stocktakes) VoidStocktake(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.VoidStocktakeRequest],
) (*connect.Response[stocktakeifacev1.VoidStocktakeResponse], error) {
	var session model.StocktakeSession
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sess, ierr := lockDraftSession(tx, req.Msg.SessionId)
		if ierr != nil {
			return ierr
		}
		now := time.Now()
		if ierr := tx.Model(sess).Updates(map[string]any{
			"status":    stocktakeStatusVoided,
			"voided_at": now,
		}).Error; ierr != nil {
			return ierr
		}
		sess.Status = stocktakeStatusVoided
		sess.VoidedAt = &now
		session = *sess
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	out, err := s.hydrateSession(ctx, &session)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.VoidStocktakeResponse{Session: out}), nil
}

func (s *Stocktakes) ListStocktakes(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.ListStocktakesRequest],
) (*connect.Response[stocktakeifacev1.ListStocktakesResponse], error) {
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
		// Scope to the caller's active warehouse (header-driven, like ListMovements).
		q = q.Where("warehouse_id = ?", warehouseID)
		if st := strings.TrimSpace(strings.ToUpper(req.Msg.Status)); st != "" {
			q = q.Where("status = ?", st)
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StocktakeSession{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.StocktakeSession
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StocktakeSession{})).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*stocktakeifacev1.StocktakeSession, 0, len(rows))
	for i := range rows {
		hydrated, err := s.hydrateSession(ctx, &rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, hydrated)
	}
	return connect.NewResponse(&stocktakeifacev1.ListStocktakesResponse{
		Sessions: out,
		Total:    int32(total),
	}), nil
}

func (s *Stocktakes) GetStocktake(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.GetStocktakeRequest],
) (*connect.Response[stocktakeifacev1.GetStocktakeResponse], error) {
	var session model.StocktakeSession
	if err := s.db.WithContext(ctx).Where("id = ?", req.Msg.Id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("stocktake not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	sessProto, err := s.hydrateSession(ctx, &session)
	if err != nil {
		return nil, err
	}
	// Load lines + denormalize batch/medicine context for display.
	type lineRow struct {
		model.StocktakeLine
		MedicineID   string `gorm:"column:medicine_id"`
		MedicineName string `gorm:"column:medicine_name"`
		MedicineSku  string `gorm:"column:medicine_sku"`
		BatchNumber  string `gorm:"column:batch_number"`
		ExpiryDate   string `gorm:"column:expiry_date"`
	}
	var rows []lineRow
	err = s.db.WithContext(ctx).
		Table("stocktake_lines AS l").
		Select(`l.*,
		        b.medicine_id AS medicine_id,
		        m.name AS medicine_name,
		        m.sku AS medicine_sku,
		        b.batch_number AS batch_number,
		        TO_CHAR(b.expiry_date, 'YYYY-MM-DD') AS expiry_date`).
		Joins("JOIN batches AS b ON b.id = l.batch_id").
		Joins("JOIN medicines AS m ON m.id = b.medicine_id").
		Where("l.session_id = ?", session.ID).
		Order("l.created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	lines := make([]*stocktakeifacev1.StocktakeLine, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, lineRowToProto(r.StocktakeLine, r.MedicineID, r.MedicineName, r.MedicineSku, r.BatchNumber, r.ExpiryDate))
	}
	return connect.NewResponse(&stocktakeifacev1.GetStocktakeResponse{
		Session: sessProto,
		Lines:   lines,
	}), nil
}

// ---------- helpers ----------

func lockDraftSession(tx *gorm.DB, id string) (*model.StocktakeSession, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id required"))
	}
	var sess model.StocktakeSession
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("stocktake not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if sess.Status != stocktakeStatusDraft {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("stocktake is %s; only DRAFT sessions can be mutated", sess.Status))
	}
	return &sess, nil
}

func (s *Stocktakes) loadLine(ctx context.Context, id string) (*stocktakeifacev1.StocktakeLine, error) {
	type lineRow struct {
		model.StocktakeLine
		MedicineID   string `gorm:"column:medicine_id"`
		MedicineName string `gorm:"column:medicine_name"`
		MedicineSku  string `gorm:"column:medicine_sku"`
		BatchNumber  string `gorm:"column:batch_number"`
		ExpiryDate   string `gorm:"column:expiry_date"`
	}
	var r lineRow
	err := s.db.WithContext(ctx).
		Table("stocktake_lines AS l").
		Select(`l.*,
		        b.medicine_id AS medicine_id,
		        m.name AS medicine_name,
		        m.sku AS medicine_sku,
		        b.batch_number AS batch_number,
		        TO_CHAR(b.expiry_date, 'YYYY-MM-DD') AS expiry_date`).
		Joins("JOIN batches AS b ON b.id = l.batch_id").
		Joins("JOIN medicines AS m ON m.id = b.medicine_id").
		Where("l.id = ?", id).
		Scan(&r).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return lineRowToProto(r.StocktakeLine, r.MedicineID, r.MedicineName, r.MedicineSku, r.BatchNumber, r.ExpiryDate), nil
}

func (s *Stocktakes) hydrateSession(
	ctx context.Context,
	sess *model.StocktakeSession,
) (*stocktakeifacev1.StocktakeSession, error) {
	type counts struct {
		Total    int32 `gorm:"column:total"`
		Counted  int32 `gorm:"column:counted"`
		Variance int32 `gorm:"column:variance"`
	}
	var c counts
	err := s.db.WithContext(ctx).
		Table("stocktake_lines").
		Select(`COUNT(*) AS total,
		        COUNT(counted_qty) AS counted,
		        COUNT(*) FILTER (WHERE counted_qty IS NOT NULL AND counted_qty <> expected_qty) AS variance`).
		Where("session_id = ?", sess.ID).
		Scan(&c).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := sessionToProto(sess, c.Total, c.Counted, c.Variance)
	// Resolve the warehouse name for display.
	if sess.WarehouseID != nil && *sess.WarehouseID != "" {
		var wh model.Warehouse
		if err := s.db.WithContext(ctx).Select("name").
			Where("id = ?", *sess.WarehouseID).First(&wh).Error; err == nil {
			out.WarehouseName = wh.Name
		}
	}
	return out, nil
}

// ---------- proto mapping ----------

func sessionToProto(s *model.StocktakeSession, total, counted, variance int32) *stocktakeifacev1.StocktakeSession {
	out := &stocktakeifacev1.StocktakeSession{
		Id:            s.ID,
		Name:          s.Name,
		Status:        s.Status,
		CreatedBy:     s.CreatedBy,
		CreatedAt:     s.CreatedAt.Unix(),
		LineCount:     total,
		CountedCount:  counted,
		VarianceCount: variance,
	}
	if s.BranchID != nil {
		out.BranchId = *s.BranchID
	}
	if s.WarehouseID != nil {
		out.WarehouseId = *s.WarehouseID
	}
	if s.CompletedAt != nil {
		out.CompletedAt = s.CompletedAt.Unix()
	}
	if s.VoidedAt != nil {
		out.VoidedAt = s.VoidedAt.Unix()
	}
	return out
}

func lineRowToProto(
	l model.StocktakeLine,
	medicineID, medicineName, medicineSku, batchNumber, expiryDate string,
) *stocktakeifacev1.StocktakeLine {
	out := &stocktakeifacev1.StocktakeLine{
		Id:              l.ID,
		SessionId:       l.SessionID,
		BatchId:         l.BatchID,
		MedicineId:      medicineID,
		MedicineName:    medicineName,
		MedicineSku:     medicineSku,
		BatchNumber:     batchNumber,
		ExpiryDate:      expiryDate,
		ExpectedQty:     l.ExpectedQty,
		Disposition:     l.Disposition,
		DispositionNote: l.DispositionNote,
	}
	if l.WriteOffKind != nil {
		out.WriteOffKind = *l.WriteOffKind
	}
	if l.CountedQty != nil {
		out.Counted = true
		out.CountedQty = *l.CountedQty
		out.Variance = *l.CountedQty - l.ExpectedQty
	}
	return out
}
