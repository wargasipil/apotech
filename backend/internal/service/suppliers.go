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
)

type Suppliers struct {
	db *gorm.DB
}

func NewSuppliers(db *gorm.DB) *Suppliers { return &Suppliers{db: db} }

func (s *Suppliers) ListSuppliers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListSuppliersRequest],
) (*connect.Response[inventoryifacev1.ListSuppliersResponse], error) {
	q := s.db.WithContext(ctx).Order("name")
	if !req.Msg.IncludeInactive {
		q = q.Where("active = ?", true)
	}
	var rows []model.Supplier
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Supplier, 0, len(rows))
	for _, r := range rows {
		out = append(out, supplierToProto(&r))
	}
	return connect.NewResponse(&inventoryifacev1.ListSuppliersResponse{Suppliers: out}), nil
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
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name required"))
	}
	sup := model.Supplier{
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
		"name":          strings.TrimSpace(req.Msg.Name),
		"contact_email": strings.TrimSpace(req.Msg.ContactEmail),
		"phone":         strings.TrimSpace(req.Msg.Phone),
	}
	if updates["name"].(string) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name required"))
	}
	if err := s.db.WithContext(ctx).Model(sup).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
		q = q.Where("name ILIKE ? OR contact_email ILIKE ? OR phone ILIKE ?", pattern, pattern, pattern)
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
		Name:         s.Name,
		ContactEmail: s.ContactEmail,
		Phone:        s.Phone,
		Active:       s.Active,
		CreatedAt:    s.CreatedAt.Unix(),
	}
}
