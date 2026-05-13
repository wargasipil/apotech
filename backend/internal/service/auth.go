package service

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	userifacev1 "github.com/apotech/backend/gen/user_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
)

type Auth struct {
	db     *gorm.DB
	issuer *auth.Issuer
}

func NewAuth(db *gorm.DB, issuer *auth.Issuer) *Auth {
	return &Auth{db: db, issuer: issuer}
}

func (a *Auth) Login(
	ctx context.Context,
	req *connect.Request[userifacev1.LoginRequest],
) (*connect.Response[userifacev1.LoginResponse], error) {
	email := strings.TrimSpace(req.Msg.Email)
	if email == "" || req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email and password required"))
	}

	var user model.User
	err := a.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !user.Active {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("account disabled"))
	}
	if err := auth.VerifyPassword(user.PasswordHash, req.Msg.Password); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}

	token, exp, err := a.issuer.Issue(user.ID, user.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userifacev1.LoginResponse{
		Token:     token,
		User:      toProto(&user),
		ExpiresAt: exp.Unix(),
	}), nil
}

func (a *Auth) Me(
	ctx context.Context,
	_ *connect.Request[userifacev1.MeRequest],
) (*connect.Response[userifacev1.MeResponse], error) {
	p, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var user model.User
	if err := a.db.WithContext(ctx).Where("id = ?", p.UserID).First(&user).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&userifacev1.MeResponse{User: toProto(&user)}), nil
}
