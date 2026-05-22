package service

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/apotech/backend/gen/warehouse_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
)

type Warehouses struct {
	db *gorm.DB
}

func NewWarehouses(db *gorm.DB) *Warehouses { return &Warehouses{db: db} }

func (w *Warehouses) ListWarehouses(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.ListWarehousesRequest],
) (*connect.Response[warehouseifacev1.ListWarehousesResponse], error) {
	q := w.db.WithContext(ctx).Order("code ASC")
	if !req.Msg.IncludeInactive {
		q = q.Where("active = ?", true)
	}
	var rows []model.Warehouse
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*warehouseifacev1.Warehouse, 0, len(rows))
	for i := range rows {
		out = append(out, warehouseToProto(&rows[i]))
	}
	return connect.NewResponse(&warehouseifacev1.ListWarehousesResponse{Warehouses: out}), nil
}

func (w *Warehouses) CreateWarehouse(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.CreateWarehouseRequest],
) (*connect.Response[warehouseifacev1.CreateWarehouseResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	code := strings.TrimSpace(strings.ToUpper(req.Msg.Code))
	name := strings.TrimSpace(req.Msg.Name)
	if code == "" || name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code and name required"))
	}
	row := model.Warehouse{
		Code:    code,
		Name:    name,
		Address: strings.TrimSpace(req.Msg.Address),
		Phone:   strings.TrimSpace(req.Msg.Phone),
		Active:  true,
	}
	err = w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		// Auto-grant the creator access so they can use it immediately.
		return tx.Save(&model.UserWarehouse{
			UserID:      caller.UserID,
			WarehouseID: row.ID,
		}).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	return connect.NewResponse(&warehouseifacev1.CreateWarehouseResponse{Warehouse: warehouseToProto(&row)}), nil
}

func (w *Warehouses) UpdateWarehouse(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.UpdateWarehouseRequest],
) (*connect.Response[warehouseifacev1.UpdateWarehouseResponse], error) {
	row, err := w.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name required"))
	}
	updates := map[string]any{
		"name":    name,
		"address": strings.TrimSpace(req.Msg.Address),
		"phone":   strings.TrimSpace(req.Msg.Phone),
	}
	if err := w.db.WithContext(ctx).Model(row).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row, err = w.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&warehouseifacev1.UpdateWarehouseResponse{Warehouse: warehouseToProto(row)}), nil
}

func (w *Warehouses) ArchiveWarehouse(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.ArchiveWarehouseRequest],
) (*connect.Response[warehouseifacev1.ArchiveWarehouseResponse], error) {
	row, err := w.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if row.IsDefault {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("cannot archive the default warehouse"))
	}
	if err := w.db.WithContext(ctx).Model(row).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row.Active = false
	return connect.NewResponse(&warehouseifacev1.ArchiveWarehouseResponse{Warehouse: warehouseToProto(row)}), nil
}

func (w *Warehouses) GrantWarehouseAccess(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.GrantWarehouseAccessRequest],
) (*connect.Response[warehouseifacev1.GrantWarehouseAccessResponse], error) {
	if req.Msg.UserId == "" || req.Msg.WarehouseId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id and warehouse_id required"))
	}
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		mem := model.UserWarehouse{
			UserID:      req.Msg.UserId,
			WarehouseID: req.Msg.WarehouseId,
			IsDefault:   req.Msg.IsDefault,
		}
		if err := tx.Save(&mem).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if req.Msg.IsDefault {
			if err := tx.Model(&model.UserWarehouse{}).
				Where("user_id = ? AND warehouse_id <> ?", req.Msg.UserId, req.Msg.WarehouseId).
				Update("is_default", false).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	return connect.NewResponse(&warehouseifacev1.GrantWarehouseAccessResponse{
		Membership: &warehouseifacev1.UserWarehouseMembership{
			UserId:      req.Msg.UserId,
			WarehouseId: req.Msg.WarehouseId,
			IsDefault:   req.Msg.IsDefault,
		},
	}), nil
}

func (w *Warehouses) RevokeWarehouseAccess(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.RevokeWarehouseAccessRequest],
) (*connect.Response[warehouseifacev1.RevokeWarehouseAccessResponse], error) {
	res := w.db.WithContext(ctx).Where("user_id = ? AND warehouse_id = ?",
		req.Msg.UserId, req.Msg.WarehouseId).Delete(&model.UserWarehouse{})
	if res.Error != nil {
		return nil, connect.NewError(connect.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("membership not found"))
	}
	return connect.NewResponse(&warehouseifacev1.RevokeWarehouseAccessResponse{}), nil
}

func (w *Warehouses) ListUserWarehouses(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.ListUserWarehousesRequest],
) (*connect.Response[warehouseifacev1.ListUserWarehousesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	target := req.Msg.UserId
	if target == "" {
		target = caller.UserID
	}
	if target != caller.UserID && caller.Role != "OWNER" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("can only list own memberships"))
	}

	var mems []model.UserWarehouse
	if err := w.db.WithContext(ctx).Where("user_id = ?", target).Find(&mems).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(mems) == 0 {
		return connect.NewResponse(&warehouseifacev1.ListUserWarehousesResponse{}), nil
	}
	ids := make([]string, 0, len(mems))
	for _, m := range mems {
		ids = append(ids, m.WarehouseID)
	}
	var whs []model.Warehouse
	if err := w.db.WithContext(ctx).Where("id IN ? AND active = ?", ids, true).Order("code ASC").
		Find(&whs).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	outMems := make([]*warehouseifacev1.UserWarehouseMembership, 0, len(mems))
	for _, m := range mems {
		outMems = append(outMems, &warehouseifacev1.UserWarehouseMembership{
			UserId:      m.UserID,
			WarehouseId: m.WarehouseID,
			IsDefault:   m.IsDefault,
		})
	}
	outWhs := make([]*warehouseifacev1.Warehouse, 0, len(whs))
	for i := range whs {
		outWhs = append(outWhs, warehouseToProto(&whs[i]))
	}
	return connect.NewResponse(&warehouseifacev1.ListUserWarehousesResponse{
		Memberships: outMems,
		Warehouses:  outWhs,
	}), nil
}

func (w *Warehouses) SetDefaultWarehouse(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.SetDefaultWarehouseRequest],
) (*connect.Response[warehouseifacev1.SetDefaultWarehouseResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	target := req.Msg.UserId
	if target == "" {
		target = caller.UserID
	}
	if target != caller.UserID && caller.Role != "OWNER" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("can only set own default"))
	}

	err = w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var mem model.UserWarehouse
		if err := tx.Where("user_id = ? AND warehouse_id = ?", target, req.Msg.WarehouseId).
			First(&mem).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodePermissionDenied,
					errors.New("user has no access to that warehouse"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&model.UserWarehouse{}).
			Where("user_id = ?", target).Update("is_default", false).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&model.UserWarehouse{}).
			Where("user_id = ? AND warehouse_id = ?", target, req.Msg.WarehouseId).
			Update("is_default", true).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	return connect.NewResponse(&warehouseifacev1.SetDefaultWarehouseResponse{
		Membership: &warehouseifacev1.UserWarehouseMembership{
			UserId:      target,
			WarehouseId: req.Msg.WarehouseId,
			IsDefault:   true,
		},
	}), nil
}

func (w *Warehouses) load(ctx context.Context, id string) (*model.Warehouse, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var row model.Warehouse
	err := w.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("warehouse not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &row, nil
}

func warehouseToProto(w *model.Warehouse) *warehouseifacev1.Warehouse {
	return &warehouseifacev1.Warehouse{
		Id:        w.ID,
		Code:      w.Code,
		Name:      w.Name,
		Address:   w.Address,
		Phone:     w.Phone,
		IsDefault: w.IsDefault,
		Active:    w.Active,
		CreatedAt: w.CreatedAt.Unix(),
	}
}

// resolveWarehouse returns the active warehouse id for the caller: the
// X-Warehouse-Id header if present, else the user's default membership, else
// the global default warehouse. It never returns empty on a healthy DB, so
// every stock movement can be stamped with a concrete warehouse.
func resolveWarehouse(ctx context.Context, db *gorm.DB, caller auth.Principal) (string, error) {
	if caller.WarehouseID != "" {
		return caller.WarehouseID, nil
	}
	var mem model.UserWarehouse
	err := db.WithContext(ctx).Where("user_id = ? AND is_default", caller.UserID).First(&mem).Error
	if err == nil {
		return mem.WarehouseID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	var def model.Warehouse
	if err := db.WithContext(ctx).Where("is_default").First(&def).Error; err != nil {
		return "", connect.NewError(connect.CodeFailedPrecondition,
			errors.New("no warehouse configured"))
	}
	return def.ID, nil
}
