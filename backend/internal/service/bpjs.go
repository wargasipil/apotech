package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	bpjsifacev1 "github.com/apotech/backend/gen/bpjs_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
	"github.com/apotech/backend/internal/sqldialect"
)

const (
	bpjsStatusDraft     = "DRAFT"
	bpjsStatusSubmitted = "SUBMITTED"
	bpjsStatusApproved  = "APPROVED"
	bpjsStatusRejected  = "REJECTED"
	bpjsStatusPaid      = "PAID"
)

func isResolvableBpjsStatus(s string) bool {
	switch s {
	case bpjsStatusApproved, bpjsStatusRejected, bpjsStatusPaid:
		return true
	}
	return false
}

type BpjsClaims struct {
	db *gorm.DB
}

func NewBpjsClaims(db *gorm.DB) *BpjsClaims { return &BpjsClaims{db: db} }

func (b *BpjsClaims) ListClaims(
	ctx context.Context,
	req *connect.Request[bpjsifacev1.ListClaimsRequest],
) (*connect.Response[bpjsifacev1.ListClaimsResponse], error) {
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if s := strings.TrimSpace(strings.ToUpper(req.Msg.Status)); s != "" {
			q = q.Where("status = ?", s)
		}
		if req.Msg.CustomerId != "" {
			q = q.Where("customer_id = ?", req.Msg.CustomerId)
		}
		return q
	}
	var total int64
	if err := applyFilters(b.db.WithContext(ctx).Model(&model.BpjsClaim{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.BpjsClaim
	if err := applyFilters(b.db.WithContext(ctx).Model(&model.BpjsClaim{})).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*bpjsifacev1.BpjsClaim, 0, len(rows))
	for i := range rows {
		out = append(out, bpjsToProto(&rows[i]))
	}
	return connect.NewResponse(&bpjsifacev1.ListClaimsResponse{
		Claims: out,
		Total:  int32(total),
	}), nil
}

func (b *BpjsClaims) GetClaim(
	ctx context.Context,
	req *connect.Request[bpjsifacev1.GetClaimRequest],
) (*connect.Response[bpjsifacev1.GetClaimResponse], error) {
	c, err := b.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&bpjsifacev1.GetClaimResponse{Claim: bpjsToProto(c)}), nil
}

func (b *BpjsClaims) CreateClaim(
	ctx context.Context,
	req *connect.Request[bpjsifacev1.CreateClaimRequest],
) (*connect.Response[bpjsifacev1.CreateClaimResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var sale model.Sale
	if err := b.db.WithContext(ctx).Where("id = ?", req.Msg.SaleId).First(&sale).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if sale.CustomerID == nil || *sale.CustomerID == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("sale must have a customer with a BPJS number"))
	}
	var cust model.Customer
	if err := b.db.WithContext(ctx).Where("id = ?", *sale.CustomerID).First(&cust).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if strings.TrimSpace(cust.BPJSNo) == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("customer has no BPJS number"))
	}
	claim := model.BpjsClaim{
		SaleID:     sale.ID,
		CustomerID: cust.ID,
		BPJSNo:     cust.BPJSNo,
		Amount:     sale.Total,
		Status:     bpjsStatusDraft,
		Note:       strings.TrimSpace(req.Msg.Note),
		CreatedBy:  caller.UserID,
	}
	if err := b.db.WithContext(ctx).Create(&claim).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bpjsifacev1.CreateClaimResponse{Claim: bpjsToProto(&claim)}), nil
}

// SubmitClaim is a stub today — it flips DRAFT -> SUBMITTED with a timestamp.
// Wire the real BPJS web service client here when credentials are available.
func (b *BpjsClaims) SubmitClaim(
	ctx context.Context,
	req *connect.Request[bpjsifacev1.SubmitClaimRequest],
) (*connect.Response[bpjsifacev1.SubmitClaimResponse], error) {
	err := b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var claim model.BpjsClaim
		err := tx.Clauses(sqldialect.LockForUpdate()).
			Where("id = ?", req.Msg.Id).First(&claim).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("claim not found"))
		}
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if claim.Status != bpjsStatusDraft {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("claim must be DRAFT to submit; is %s", claim.Status))
		}
		now := time.Now()
		// TODO: real BPJS HTTP call goes here.
		return tx.Model(&claim).Updates(map[string]any{
			"status":       bpjsStatusSubmitted,
			"submitted_at": now,
		}).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	c, err := b.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&bpjsifacev1.SubmitClaimResponse{Claim: bpjsToProto(c)}), nil
}

func (b *BpjsClaims) ResolveClaim(
	ctx context.Context,
	req *connect.Request[bpjsifacev1.ResolveClaimRequest],
) (*connect.Response[bpjsifacev1.ResolveClaimResponse], error) {
	status := strings.ToUpper(strings.TrimSpace(req.Msg.Status))
	if !isResolvableBpjsStatus(status) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("status must be APPROVED, REJECTED, or PAID"))
	}
	err := b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var claim model.BpjsClaim
		err := tx.Clauses(sqldialect.LockForUpdate()).
			Where("id = ?", req.Msg.Id).First(&claim).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("claim not found"))
		}
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if claim.Status != bpjsStatusSubmitted && claim.Status != bpjsStatusApproved {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only SUBMITTED or APPROVED claims can be resolved; is %s", claim.Status))
		}
		now := time.Now()
		return tx.Model(&claim).Updates(map[string]any{
			"status":       status,
			"external_ref": strings.TrimSpace(req.Msg.ExternalRef),
			"note":         strings.TrimSpace(req.Msg.Note),
			"resolved_at":  now,
		}).Error
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	c, err := b.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&bpjsifacev1.ResolveClaimResponse{Claim: bpjsToProto(c)}), nil
}

func (b *BpjsClaims) load(ctx context.Context, id string) (*model.BpjsClaim, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var c model.BpjsClaim
	err := b.db.WithContext(ctx).Where("id = ?", id).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("claim not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &c, nil
}

func bpjsToProto(c *model.BpjsClaim) *bpjsifacev1.BpjsClaim {
	out := &bpjsifacev1.BpjsClaim{
		Id:          c.ID,
		SaleId:      c.SaleID,
		CustomerId:  c.CustomerID,
		BpjsNo:      c.BPJSNo,
		Status:      c.Status,
		Amount:      c.Amount,
		ExternalRef: c.ExternalRef,
		Note:        c.Note,
		CreatedAt:   c.CreatedAt.Unix(),
	}
	if c.SubmittedAt != nil {
		out.SubmittedAt = c.SubmittedAt.Unix()
	}
	if c.ResolvedAt != nil {
		out.ResolvedAt = c.ResolvedAt.Unix()
	}
	return out
}
