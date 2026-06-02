package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	unitifacev1 "github.com/apotech/backend/gen/unit_iface/v1"
	"github.com/apotech/backend/internal/model"
)

type Units struct {
	db *gorm.DB
}

func NewUnits(db *gorm.DB) *Units { return &Units{db: db} }

func (u *Units) ListUnitBases(
	ctx context.Context,
	req *connect.Request[unitifacev1.ListUnitBasesRequest],
) (*connect.Response[unitifacev1.ListUnitBasesResponse], error) {
	var bases []model.UnitBase
	q := u.db.WithContext(ctx).Order("name")
	if !req.Msg.IncludeInactive {
		q = q.Where("active")
	}
	if err := q.Find(&bases).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(bases) == 0 {
		return connect.NewResponse(&unitifacev1.ListUnitBasesResponse{}), nil
	}
	ids := make([]string, 0, len(bases))
	for _, b := range bases {
		ids = append(ids, b.ID)
	}
	var derivs []model.UnitDerivative
	dq := u.db.WithContext(ctx).Where("base_unit_id IN ?", ids).Order("sort_order, name")
	if !req.Msg.IncludeInactive {
		dq = dq.Where("active")
	}
	if err := dq.Find(&derivs).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	byBase := make(map[string][]*unitifacev1.UnitDerivative, len(bases))
	for i := range derivs {
		d := &derivs[i]
		byBase[d.BaseUnitID] = append(byBase[d.BaseUnitID], derivativeToProto(d))
	}
	out := make([]*unitifacev1.UnitBase, 0, len(bases))
	for i := range bases {
		b := &bases[i]
		out = append(out, &unitifacev1.UnitBase{
			Id:          b.ID,
			Name:        b.Name,
			Active:      b.Active,
			CreatedAt:   b.CreatedAt.Unix(),
			Derivatives: byBase[b.ID],
		})
	}
	return connect.NewResponse(&unitifacev1.ListUnitBasesResponse{Bases: out}), nil
}

func (u *Units) CreateUnitBase(
	ctx context.Context,
	req *connect.Request[unitifacev1.CreateUnitBaseRequest],
) (*connect.Response[unitifacev1.CreateUnitBaseResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	row := model.UnitBase{Name: name, Active: true}
	if err := u.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("base unit %q already exists or DB error: %w", name, err))
	}
	return connect.NewResponse(&unitifacev1.CreateUnitBaseResponse{
		Base: baseToProto(&row, nil),
	}), nil
}

func (u *Units) UpdateUnitBase(
	ctx context.Context,
	req *connect.Request[unitifacev1.UpdateUnitBaseRequest],
) (*connect.Response[unitifacev1.UpdateUnitBaseResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	var row model.UnitBase
	if err := u.db.WithContext(ctx).First(&row, "id = ?", req.Msg.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("base unit not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := u.db.WithContext(ctx).Model(&row).Update("name", name).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("base unit %q already exists or DB error: %w", name, err))
	}
	row.Name = name
	return connect.NewResponse(&unitifacev1.UpdateUnitBaseResponse{
		Base: baseToProto(&row, nil),
	}), nil
}

func (u *Units) ArchiveUnitBase(
	ctx context.Context,
	req *connect.Request[unitifacev1.ArchiveUnitBaseRequest],
) (*connect.Response[unitifacev1.ArchiveUnitBaseResponse], error) {
	var row model.UnitBase
	if err := u.db.WithContext(ctx).First(&row, "id = ?", req.Msg.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("base unit not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := u.db.WithContext(ctx).Model(&row).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row.Active = false
	return connect.NewResponse(&unitifacev1.ArchiveUnitBaseResponse{
		Base: baseToProto(&row, nil),
	}), nil
}

func (u *Units) CreateUnitDerivative(
	ctx context.Context,
	req *connect.Request[unitifacev1.CreateUnitDerivativeRequest],
) (*connect.Response[unitifacev1.CreateUnitDerivativeResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	if req.Msg.Factor <= 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("factor must be > 1"))
	}
	// Confirm the base exists.
	var base model.UnitBase
	if err := u.db.WithContext(ctx).First(&base, "id = ?", req.Msg.BaseUnitId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("base unit not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row := model.UnitDerivative{
		BaseUnitID: req.Msg.BaseUnitId,
		Name:       name,
		Factor:     req.Msg.Factor,
		SortOrder:  req.Msg.SortOrder,
		Active:     true,
	}
	if err := u.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("derivative %q already exists for this base or DB error: %w", name, err))
	}
	return connect.NewResponse(&unitifacev1.CreateUnitDerivativeResponse{
		Derivative: derivativeToProto(&row),
	}), nil
}

func (u *Units) UpdateUnitDerivative(
	ctx context.Context,
	req *connect.Request[unitifacev1.UpdateUnitDerivativeRequest],
) (*connect.Response[unitifacev1.UpdateUnitDerivativeResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	if req.Msg.Factor <= 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("factor must be > 1"))
	}
	var row model.UnitDerivative
	if err := u.db.WithContext(ctx).First(&row, "id = ?", req.Msg.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("derivative not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := u.db.WithContext(ctx).Model(&row).Updates(map[string]any{
		"name":       name,
		"factor":     req.Msg.Factor,
		"sort_order": req.Msg.SortOrder,
	}).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("derivative %q already exists for this base or DB error: %w", name, err))
	}
	row.Name = name
	row.Factor = req.Msg.Factor
	row.SortOrder = req.Msg.SortOrder
	return connect.NewResponse(&unitifacev1.UpdateUnitDerivativeResponse{
		Derivative: derivativeToProto(&row),
	}), nil
}

func (u *Units) ArchiveUnitDerivative(
	ctx context.Context,
	req *connect.Request[unitifacev1.ArchiveUnitDerivativeRequest],
) (*connect.Response[unitifacev1.ArchiveUnitDerivativeResponse], error) {
	var row model.UnitDerivative
	if err := u.db.WithContext(ctx).First(&row, "id = ?", req.Msg.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("derivative not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := u.db.WithContext(ctx).Model(&row).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row.Active = false
	return connect.NewResponse(&unitifacev1.ArchiveUnitDerivativeResponse{
		Derivative: derivativeToProto(&row),
	}), nil
}

func baseToProto(b *model.UnitBase, derivs []*unitifacev1.UnitDerivative) *unitifacev1.UnitBase {
	return &unitifacev1.UnitBase{
		Id:          b.ID,
		Name:        b.Name,
		Active:      b.Active,
		CreatedAt:   b.CreatedAt.Unix(),
		Derivatives: derivs,
	}
}

func derivativeToProto(d *model.UnitDerivative) *unitifacev1.UnitDerivative {
	return &unitifacev1.UnitDerivative{
		Id:         d.ID,
		BaseUnitId: d.BaseUnitID,
		Name:       d.Name,
		Factor:     d.Factor,
		SortOrder:  d.SortOrder,
		Active:     d.Active,
	}
}
