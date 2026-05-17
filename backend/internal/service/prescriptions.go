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

	prescriptionifacev1 "github.com/apotech/backend/gen/prescription_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
)

const (
	rxStatusActive    = "ACTIVE"
	rxStatusDispensed = "DISPENSED"
	rxStatusExpired   = "EXPIRED"
	rxStatusVoided    = "VOIDED"

	defaultRxValidityDays = 90
)

type Prescriptions struct {
	db *gorm.DB
}

func NewPrescriptions(db *gorm.DB) *Prescriptions { return &Prescriptions{db: db} }

func (p *Prescriptions) ListPrescriptions(
	ctx context.Context,
	req *connect.Request[prescriptionifacev1.ListPrescriptionsRequest],
) (*connect.Response[prescriptionifacev1.ListPrescriptionsResponse], error) {
	q := p.db.WithContext(ctx).Preload("Items").Order("created_at DESC")
	if req.Msg.CustomerId != "" {
		q = q.Where("customer_id = ?", req.Msg.CustomerId)
	}
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q = q.Limit(limit)

	var rows []model.Prescription
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Filter by computed status client-side (cheap; lists are short).
	filter := strings.TrimSpace(strings.ToUpper(req.Msg.Status))
	out := make([]*prescriptionifacev1.Prescription, 0, len(rows))
	for i := range rows {
		status := computeRxStatus(&rows[i], time.Now())
		if filter != "" && status != filter {
			continue
		}
		out = append(out, rxToProto(&rows[i], status))
	}
	return connect.NewResponse(&prescriptionifacev1.ListPrescriptionsResponse{Prescriptions: out}), nil
}

func (p *Prescriptions) GetPrescription(
	ctx context.Context,
	req *connect.Request[prescriptionifacev1.GetPrescriptionRequest],
) (*connect.Response[prescriptionifacev1.GetPrescriptionResponse], error) {
	rx, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	status := computeRxStatus(rx, time.Now())
	return connect.NewResponse(&prescriptionifacev1.GetPrescriptionResponse{
		Prescription: rxToProto(rx, status),
	}), nil
}

func (p *Prescriptions) CreatePrescription(
	ctx context.Context,
	req *connect.Request[prescriptionifacev1.CreatePrescriptionRequest],
) (*connect.Response[prescriptionifacev1.CreatePrescriptionResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.CustomerId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("customer_id required"))
	}
	if strings.TrimSpace(req.Msg.IssuerName) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("issuer_name required"))
	}
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item required"))
	}

	issued, err := parseDateRequired(req.Msg.IssuedAt, "issued_at")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	expires, err := parseDateMaybe(req.Msg.ExpiresAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if expires == nil {
		d := issued.AddDate(0, 0, defaultRxValidityDays)
		expires = &d
	}
	if expires.Before(issued) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("expires_at must be on/after issued_at"))
	}

	var rx model.Prescription
	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rx = model.Prescription{
			CustomerID: req.Msg.CustomerId,
			IssuerName: strings.TrimSpace(req.Msg.IssuerName),
			IssuedAt:   issued,
			ExpiresAt:  *expires,
			Note:       strings.TrimSpace(req.Msg.Note),
			Status:     rxStatusActive,
			CreatedBy:  caller.UserID,
		}
		rxNo, err := assignRxNo(tx, time.Now())
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		rx.RxNo = &rxNo
		if err := tx.Create(&rx).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		items := make([]model.PrescriptionItem, 0, len(req.Msg.Items))
		for _, in := range req.Msg.Items {
			if in.PrescribedQty <= 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("prescribed_qty must be > 0"))
			}
			items = append(items, model.PrescriptionItem{
				PrescriptionID:     rx.ID,
				MedicineID:         in.MedicineId,
				PrescribedQty:      in.PrescribedQty,
				DispensedQty:       0,
				DosageInstructions: strings.TrimSpace(in.DosageInstructions),
				Note:               strings.TrimSpace(in.Note),
			})
		}
		if err := tx.Create(&items).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}

	full, err := p.loadFull(ctx, rx.ID)
	if err != nil {
		return nil, err
	}
	status := computeRxStatus(full, time.Now())
	return connect.NewResponse(&prescriptionifacev1.CreatePrescriptionResponse{
		Prescription: rxToProto(full, status),
	}), nil
}

func (p *Prescriptions) UpdatePrescription(
	ctx context.Context,
	req *connect.Request[prescriptionifacev1.UpdatePrescriptionRequest],
) (*connect.Response[prescriptionifacev1.UpdatePrescriptionResponse], error) {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rx, err := p.lockByID(tx, req.Msg.Id)
		if err != nil {
			return err
		}
		if rx.Status != rxStatusActive {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only ACTIVE prescriptions can be edited; this one is %s", rx.Status))
		}
		// Block edits once any dispensing has happened.
		var dispensed int64
		if err := tx.Model(&model.PrescriptionItem{}).
			Where("prescription_id = ? AND dispensed_qty > 0", rx.ID).
			Count(&dispensed).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if dispensed > 0 {
			return connect.NewError(connect.CodeFailedPrecondition,
				errors.New("cannot edit a prescription that has already been partially dispensed"))
		}

		issuer := strings.TrimSpace(req.Msg.IssuerName)
		if issuer == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("issuer_name required"))
		}
		issued, err := parseDateRequired(req.Msg.IssuedAt, "issued_at")
		if err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		}
		expires, err := parseDateMaybe(req.Msg.ExpiresAt)
		if err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		}
		if expires == nil {
			d := issued.AddDate(0, 0, defaultRxValidityDays)
			expires = &d
		}
		if expires.Before(issued) {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("expires_at must be on/after issued_at"))
		}

		updates := map[string]any{
			"issuer_name": issuer,
			"issued_at":   issued,
			"expires_at":  *expires,
			"note":        strings.TrimSpace(req.Msg.Note),
		}
		if err := tx.Model(rx).Updates(updates).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		if len(req.Msg.Items) > 0 {
			if err := tx.Where("prescription_id = ?", rx.ID).Delete(&model.PrescriptionItem{}).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			items := make([]model.PrescriptionItem, 0, len(req.Msg.Items))
			for _, in := range req.Msg.Items {
				if in.PrescribedQty <= 0 {
					return connect.NewError(connect.CodeInvalidArgument, errors.New("prescribed_qty must be > 0"))
				}
				items = append(items, model.PrescriptionItem{
					PrescriptionID:     rx.ID,
					MedicineID:         in.MedicineId,
					PrescribedQty:      in.PrescribedQty,
					DispensedQty:       0,
					DosageInstructions: strings.TrimSpace(in.DosageInstructions),
					Note:               strings.TrimSpace(in.Note),
				})
			}
			if err := tx.Create(&items).Error; err != nil {
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
	status := computeRxStatus(full, time.Now())
	return connect.NewResponse(&prescriptionifacev1.UpdatePrescriptionResponse{
		Prescription: rxToProto(full, status),
	}), nil
}

func (p *Prescriptions) VoidPrescription(
	ctx context.Context,
	req *connect.Request[prescriptionifacev1.VoidPrescriptionRequest],
) (*connect.Response[prescriptionifacev1.VoidPrescriptionResponse], error) {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rx, err := p.lockByID(tx, req.Msg.Id)
		if err != nil {
			return err
		}
		if rx.Status == rxStatusVoided {
			return nil // idempotent
		}
		return tx.Model(rx).Update("status", rxStatusVoided).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}

	full, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	status := computeRxStatus(full, time.Now())
	return connect.NewResponse(&prescriptionifacev1.VoidPrescriptionResponse{
		Prescription: rxToProto(full, status),
	}), nil
}

// ---------- Helpers (also used by sales.go) ----------

func (p *Prescriptions) lockByID(tx *gorm.DB, id string) (*model.Prescription, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var rx model.Prescription
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&rx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("prescription not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &rx, nil
}

func (p *Prescriptions) loadFull(ctx context.Context, id string) (*model.Prescription, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var rx model.Prescription
	err := p.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&rx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("prescription not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &rx, nil
}

func parseDateRequired(s, field string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("%s required", field)
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD: %w", field, err)
	}
	return t, nil
}

func assignRxNo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	var counter model.RxCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.RxCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := tx.Model(&model.RxCounter{}).
		Where("year = ?", year).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("RX-%d-%04d", year, counter.LastSeq), nil
}

// computeRxStatus derives status from stored fields. The DB only stores
// ACTIVE / VOIDED; DISPENSED and EXPIRED are computed live.
func computeRxStatus(rx *model.Prescription, now time.Time) string {
	if rx.Status == rxStatusVoided {
		return rxStatusVoided
	}
	allDispensed := len(rx.Items) > 0
	for _, it := range rx.Items {
		if it.DispensedQty < it.PrescribedQty {
			allDispensed = false
			break
		}
	}
	if allDispensed {
		return rxStatusDispensed
	}
	if now.After(rx.ExpiresAt.AddDate(0, 0, 1)) {
		// Compare against the day AFTER ExpiresAt so a script issued + expiring
		// the same day stays valid until end-of-day.
		return rxStatusExpired
	}
	return rxStatusActive
}

// ---------- Proto mapping ----------

func rxToProto(rx *model.Prescription, status string) *prescriptionifacev1.Prescription {
	out := &prescriptionifacev1.Prescription{
		Id:         rx.ID,
		CustomerId: rx.CustomerID,
		IssuerName: rx.IssuerName,
		IssuedAt:   rx.IssuedAt.Format("2006-01-02"),
		ExpiresAt:  rx.ExpiresAt.Format("2006-01-02"),
		Note:       rx.Note,
		Status:     status,
		CreatedBy:  rx.CreatedBy,
		CreatedAt:  rx.CreatedAt.Unix(),
	}
	if rx.RxNo != nil {
		out.RxNo = *rx.RxNo
	}
	if rx.BranchID != nil {
		out.BranchId = *rx.BranchID
	}
	for i := range rx.Items {
		out.Items = append(out.Items, rxItemToProto(&rx.Items[i]))
	}
	return out
}

func rxItemToProto(it *model.PrescriptionItem) *prescriptionifacev1.PrescriptionItem {
	return &prescriptionifacev1.PrescriptionItem{
		Id:                 it.ID,
		PrescriptionId:     it.PrescriptionID,
		MedicineId:         it.MedicineID,
		PrescribedQty:      it.PrescribedQty,
		DispensedQty:       it.DispensedQty,
		DosageInstructions: it.DosageInstructions,
		Note:               it.Note,
	}
}
