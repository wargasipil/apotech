package service

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	branchifacev1 "github.com/apotech/backend/gen/branch_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
)

type Branches struct {
	db *gorm.DB
}

func NewBranches(db *gorm.DB) *Branches { return &Branches{db: db} }

func (b *Branches) ListBranches(
	ctx context.Context,
	req *connect.Request[branchifacev1.ListBranchesRequest],
) (*connect.Response[branchifacev1.ListBranchesResponse], error) {
	q := b.db.WithContext(ctx).Order("code ASC")
	if !req.Msg.IncludeInactive {
		q = q.Where("active = ?", true)
	}
	var rows []model.Branch
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*branchifacev1.Branch, 0, len(rows))
	for i := range rows {
		out = append(out, branchToProto(&rows[i]))
	}
	return connect.NewResponse(&branchifacev1.ListBranchesResponse{Branches: out}), nil
}

func (b *Branches) CreateBranch(
	ctx context.Context,
	req *connect.Request[branchifacev1.CreateBranchRequest],
) (*connect.Response[branchifacev1.CreateBranchResponse], error) {
	code := strings.TrimSpace(strings.ToUpper(req.Msg.Code))
	name := strings.TrimSpace(req.Msg.Name)
	if code == "" || name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code and name required"))
	}
	row := model.Branch{
		Code:    code,
		Name:    name,
		Address: strings.TrimSpace(req.Msg.Address),
		Phone:   strings.TrimSpace(req.Msg.Phone),
		Active:  true,
	}
	if err := b.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&branchifacev1.CreateBranchResponse{Branch: branchToProto(&row)}), nil
}

func (b *Branches) UpdateBranch(
	ctx context.Context,
	req *connect.Request[branchifacev1.UpdateBranchRequest],
) (*connect.Response[branchifacev1.UpdateBranchResponse], error) {
	row, err := b.load(ctx, req.Msg.Id)
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
	if err := b.db.WithContext(ctx).Model(row).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row, err = b.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&branchifacev1.UpdateBranchResponse{Branch: branchToProto(row)}), nil
}

func (b *Branches) ArchiveBranch(
	ctx context.Context,
	req *connect.Request[branchifacev1.ArchiveBranchRequest],
) (*connect.Response[branchifacev1.ArchiveBranchResponse], error) {
	row, err := b.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := b.db.WithContext(ctx).Model(row).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row.Active = false
	return connect.NewResponse(&branchifacev1.ArchiveBranchResponse{Branch: branchToProto(row)}), nil
}

func (b *Branches) GrantBranchAccess(
	ctx context.Context,
	req *connect.Request[branchifacev1.GrantBranchAccessRequest],
) (*connect.Response[branchifacev1.GrantBranchAccessResponse], error) {
	if req.Msg.UserId == "" || req.Msg.BranchId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id and branch_id required"))
	}
	err := b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Upsert membership.
		mem := model.UserBranch{
			UserID:    req.Msg.UserId,
			BranchID:  req.Msg.BranchId,
			IsDefault: req.Msg.IsDefault,
		}
		if err := tx.Save(&mem).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if req.Msg.IsDefault {
			// Clear other defaults for this user.
			if err := tx.Model(&model.UserBranch{}).
				Where("user_id = ? AND branch_id <> ?", req.Msg.UserId, req.Msg.BranchId).
				Update("is_default", false).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	return connect.NewResponse(&branchifacev1.GrantBranchAccessResponse{
		Membership: &branchifacev1.UserBranchMembership{
			UserId:    req.Msg.UserId,
			BranchId:  req.Msg.BranchId,
			IsDefault: req.Msg.IsDefault,
		},
	}), nil
}

func (b *Branches) RevokeBranchAccess(
	ctx context.Context,
	req *connect.Request[branchifacev1.RevokeBranchAccessRequest],
) (*connect.Response[branchifacev1.RevokeBranchAccessResponse], error) {
	res := b.db.WithContext(ctx).Where("user_id = ? AND branch_id = ?",
		req.Msg.UserId, req.Msg.BranchId).Delete(&model.UserBranch{})
	if res.Error != nil {
		return nil, connect.NewError(connect.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("membership not found"))
	}
	return connect.NewResponse(&branchifacev1.RevokeBranchAccessResponse{}), nil
}

func (b *Branches) ListUserBranches(
	ctx context.Context,
	req *connect.Request[branchifacev1.ListUserBranchesRequest],
) (*connect.Response[branchifacev1.ListUserBranchesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	target := req.Msg.UserId
	if target == "" {
		target = caller.UserID
	}
	// Non-owners can only ask about themselves.
	if target != caller.UserID && caller.Role != "OWNER" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("can only list own memberships"))
	}

	var mems []model.UserBranch
	if err := b.db.WithContext(ctx).Where("user_id = ?", target).Find(&mems).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(mems) == 0 {
		return connect.NewResponse(&branchifacev1.ListUserBranchesResponse{}), nil
	}
	ids := make([]string, 0, len(mems))
	for _, m := range mems {
		ids = append(ids, m.BranchID)
	}
	var branches []model.Branch
	if err := b.db.WithContext(ctx).Where("id IN ?", ids).Find(&branches).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	outMems := make([]*branchifacev1.UserBranchMembership, 0, len(mems))
	for _, m := range mems {
		outMems = append(outMems, &branchifacev1.UserBranchMembership{
			UserId:    m.UserID,
			BranchId:  m.BranchID,
			IsDefault: m.IsDefault,
		})
	}
	outBranches := make([]*branchifacev1.Branch, 0, len(branches))
	for i := range branches {
		outBranches = append(outBranches, branchToProto(&branches[i]))
	}
	return connect.NewResponse(&branchifacev1.ListUserBranchesResponse{
		Memberships: outMems,
		Branches:    outBranches,
	}), nil
}

func (b *Branches) SetDefaultBranch(
	ctx context.Context,
	req *connect.Request[branchifacev1.SetDefaultBranchRequest],
) (*connect.Response[branchifacev1.SetDefaultBranchResponse], error) {
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

	err = b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var mem model.UserBranch
		if err := tx.Where("user_id = ? AND branch_id = ?", target, req.Msg.BranchId).
			First(&mem).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodePermissionDenied,
					errors.New("user has no access to that branch"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&model.UserBranch{}).
			Where("user_id = ?", target).Update("is_default", false).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&model.UserBranch{}).
			Where("user_id = ? AND branch_id = ?", target, req.Msg.BranchId).
			Update("is_default", true).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	return connect.NewResponse(&branchifacev1.SetDefaultBranchResponse{
		Membership: &branchifacev1.UserBranchMembership{
			UserId:    target,
			BranchId:  req.Msg.BranchId,
			IsDefault: true,
		},
	}), nil
}

func (b *Branches) load(ctx context.Context, id string) (*model.Branch, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var row model.Branch
	err := b.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("branch not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &row, nil
}

func branchToProto(b *model.Branch) *branchifacev1.Branch {
	return &branchifacev1.Branch{
		Id:        b.ID,
		Code:      b.Code,
		Name:      b.Name,
		Address:   b.Address,
		Phone:     b.Phone,
		Active:    b.Active,
		CreatedAt: b.CreatedAt.Unix(),
	}
}
