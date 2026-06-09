package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	"github.com/apotech/backend/internal/model"
	"github.com/apotech/backend/internal/sqldialect"
)

type Suppliers struct {
	db *gorm.DB
}

func NewSuppliers(db *gorm.DB) *Suppliers { return &Suppliers{db: db} }

func (s *Suppliers) ListSuppliers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListSuppliersRequest],
) (*connect.Response[inventoryifacev1.ListSuppliersResponse], error) {
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	query := strings.TrimSpace(req.Msg.Query)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if !req.Msg.IncludeInactive {
			q = q.Where("active = ?", true)
		}
		if query != "" {
			pattern := "%" + query + "%"
			q = q.Where(sqldialect.ILikeAny("name", "code"), pattern, pattern)
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Supplier{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Supplier
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Supplier{})).
		Order("name").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Supplier, 0, len(rows))
	for _, r := range rows {
		out = append(out, supplierToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.ListSuppliersResponse{
		Suppliers: out,
		Total:     int32(total),
	}), nil
}

func (s *Suppliers) GetSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetSupplierRequest],
) (*connect.Response[inventoryifacev1.GetSupplierResponse], error) {
	sup, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.GetSupplierResponse{Supplier: supplierToProto(sup)}), nil
}

func (s *Suppliers) CreateSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateSupplierRequest],
) (*connect.Response[inventoryifacev1.CreateSupplierResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	code := strings.ToUpper(strings.TrimSpace(req.Msg.Code))
	if name == "" || code == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code and name required"))
	}
	sup := model.Supplier{
		Code:         code,
		Name:         name,
		ContactEmail: strings.TrimSpace(req.Msg.ContactEmail),
		Phone:        strings.TrimSpace(req.Msg.Phone),
		Active:       true,
	}
	if err := s.db.WithContext(ctx).Create(&sup).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("create supplier: %w", err))
	}
	return connect.NewResponse(&inventoryifacev1.CreateSupplierResponse{Supplier: supplierToProto(&sup)}), nil
}

func (s *Suppliers) UpdateSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateSupplierRequest],
) (*connect.Response[inventoryifacev1.UpdateSupplierResponse], error) {
	sup, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"code":          strings.ToUpper(strings.TrimSpace(req.Msg.Code)),
		"name":          strings.TrimSpace(req.Msg.Name),
		"contact_email": strings.TrimSpace(req.Msg.ContactEmail),
		"phone":         strings.TrimSpace(req.Msg.Phone),
	}
	if updates["name"].(string) == "" || updates["code"].(string) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code and name required"))
	}
	if err := s.db.WithContext(ctx).Model(sup).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("update supplier: %w", err))
	}
	return connect.NewResponse(&inventoryifacev1.UpdateSupplierResponse{Supplier: supplierToProto(sup)}), nil
}

func (s *Suppliers) SearchSuppliers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SearchSuppliersRequest],
) (*connect.Response[inventoryifacev1.SearchSuppliersResponse], error) {
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := s.db.WithContext(ctx).Where("active = ?", true).Order("name").Limit(limit)
	if query != "" {
		pattern := "%" + query + "%"
		q = q.Where(sqldialect.ILikeAny("name", "code", "contact_email", "phone"),
			pattern, pattern, pattern, pattern)
	}
	var rows []model.Supplier
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Supplier, 0, len(rows))
	for _, r := range rows {
		out = append(out, supplierToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.SearchSuppliersResponse{Suppliers: out}), nil
}

// ResolveSuppliers returns minimal display refs for a set of ids. Unknown ids
// are omitted; empty input returns an empty list. No enrich, no preload.
func (s *Suppliers) ResolveSuppliers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ResolveSuppliersRequest],
) (*connect.Response[inventoryifacev1.ResolveSuppliersResponse], error) {
	ids := dedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&inventoryifacev1.ResolveSuppliersResponse{}), nil
	}
	type row struct {
		ID   string `gorm:"column:id"`
		Code string `gorm:"column:code"`
		Name string `gorm:"column:name"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Model(&model.Supplier{}).
		Select("id, code, name").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.SupplierRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.SupplierRef{Id: r.ID, Code: r.Code, Name: r.Name})
	}
	return connect.NewResponse(&inventoryifacev1.ResolveSuppliersResponse{Suppliers: out}), nil
}

func (s *Suppliers) ArchiveSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ArchiveSupplierRequest],
) (*connect.Response[inventoryifacev1.ArchiveSupplierResponse], error) {
	sup, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(sup).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	sup.Active = false
	return connect.NewResponse(&inventoryifacev1.ArchiveSupplierResponse{Supplier: supplierToProto(sup)}), nil
}

func (s *Suppliers) load(ctx context.Context, id string) (*model.Supplier, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var sup model.Supplier
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&sup).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("supplier %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &sup, nil
}

func supplierToProto(s *model.Supplier) *inventoryifacev1.Supplier {
	return &inventoryifacev1.Supplier{
		Id:           s.ID,
		Code:         s.Code,
		Name:         s.Name,
		ContactEmail: s.ContactEmail,
		Phone:        s.Phone,
		Active:       s.Active,
		CreatedAt:    s.CreatedAt.Unix(),
	}
}
