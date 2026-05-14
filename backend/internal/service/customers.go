package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	customerifacev1 "github.com/apotech/backend/gen/customer_iface/v1"
	"github.com/apotech/backend/internal/model"
)

type Customers struct {
	db *gorm.DB
}

func NewCustomers(db *gorm.DB) *Customers { return &Customers{db: db} }

func (c *Customers) ListCustomers(
	ctx context.Context,
	req *connect.Request[customerifacev1.ListCustomersRequest],
) (*connect.Response[customerifacev1.ListCustomersResponse], error) {
	q := c.db.WithContext(ctx).Order("name")
	if !req.Msg.IncludeInactive {
		q = q.Where("active = ?", true)
	}
	var rows []model.Customer
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*customerifacev1.Customer, 0, len(rows))
	for _, r := range rows {
		out = append(out, customerToProto(&r))
	}
	return connect.NewResponse(&customerifacev1.ListCustomersResponse{Customers: out}), nil
}

func (c *Customers) GetCustomer(
	ctx context.Context,
	req *connect.Request[customerifacev1.GetCustomerRequest],
) (*connect.Response[customerifacev1.GetCustomerResponse], error) {
	cust, err := c.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&customerifacev1.GetCustomerResponse{Customer: customerToProto(cust)}), nil
}

func (c *Customers) SearchCustomers(
	ctx context.Context,
	req *connect.Request[customerifacev1.SearchCustomersRequest],
) (*connect.Response[customerifacev1.SearchCustomersResponse], error) {
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	q := c.db.WithContext(ctx).Where("active = ?", true).Order("name").Limit(limit)
	if query != "" {
		pattern := "%" + query + "%"
		q = q.Where("name ILIKE ? OR phone ILIKE ? OR bpjs_no ILIKE ?", pattern, pattern, pattern)
	}

	var rows []model.Customer
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*customerifacev1.Customer, 0, len(rows))
	for _, r := range rows {
		out = append(out, customerToProto(&r))
	}
	return connect.NewResponse(&customerifacev1.SearchCustomersResponse{Customers: out}), nil
}

func (c *Customers) CreateCustomer(
	ctx context.Context,
	req *connect.Request[customerifacev1.CreateCustomerRequest],
) (*connect.Response[customerifacev1.CreateCustomerResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name required"))
	}
	row := model.Customer{
		Name:   name,
		Phone:  strings.TrimSpace(req.Msg.Phone),
		BPJSNo: strings.TrimSpace(req.Msg.BpjsNo),
		Notes:  strings.TrimSpace(req.Msg.Notes),
		Active: true,
	}
	if err := c.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create customer: %w", err))
	}
	return connect.NewResponse(&customerifacev1.CreateCustomerResponse{Customer: customerToProto(&row)}), nil
}

func (c *Customers) UpdateCustomer(
	ctx context.Context,
	req *connect.Request[customerifacev1.UpdateCustomerRequest],
) (*connect.Response[customerifacev1.UpdateCustomerResponse], error) {
	cust, err := c.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name required"))
	}
	updates := map[string]any{
		"name":    name,
		"phone":   strings.TrimSpace(req.Msg.Phone),
		"bpjs_no": strings.TrimSpace(req.Msg.BpjsNo),
		"notes":   strings.TrimSpace(req.Msg.Notes),
	}
	if err := c.db.WithContext(ctx).Model(cust).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cust, err = c.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&customerifacev1.UpdateCustomerResponse{Customer: customerToProto(cust)}), nil
}

func (c *Customers) ArchiveCustomer(
	ctx context.Context,
	req *connect.Request[customerifacev1.ArchiveCustomerRequest],
) (*connect.Response[customerifacev1.ArchiveCustomerResponse], error) {
	cust, err := c.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := c.db.WithContext(ctx).Model(cust).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cust.Active = false
	return connect.NewResponse(&customerifacev1.ArchiveCustomerResponse{Customer: customerToProto(cust)}), nil
}

func (c *Customers) load(ctx context.Context, id string) (*model.Customer, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var row model.Customer
	err := c.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("customer %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &row, nil
}

func customerToProto(c *model.Customer) *customerifacev1.Customer {
	return &customerifacev1.Customer{
		Id:        c.ID,
		Name:      c.Name,
		Phone:     c.Phone,
		BpjsNo:    c.BPJSNo,
		Notes:     c.Notes,
		Active:    c.Active,
		CreatedAt: c.CreatedAt.Unix(),
	}
}
