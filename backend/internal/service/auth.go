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
	db       *gorm.DB
	access   *auth.Issuer
	refresh  *auth.RefreshIssuer
	limiter  *auth.LoginLimiter
}

func NewAuth(
	db *gorm.DB,
	access *auth.Issuer,
	refresh *auth.RefreshIssuer,
	limiter *auth.LoginLimiter,
) *Auth {
	return &Auth{db: db, access: access, refresh: refresh, limiter: limiter}
}

func (a *Auth) Login(
	ctx context.Context,
	req *connect.Request[userifacev1.LoginRequest],
) (*connect.Response[userifacev1.LoginResponse], error) {
	email := strings.TrimSpace(req.Msg.Email)
	if email == "" || req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email and password required"))
	}

	if a.limiter != nil && !a.limiter.Allow(email) {
		return nil, connect.NewError(connect.CodeResourceExhausted,
			errors.New("too many login attempts; try again in a minute"))
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

	accessTok, accessExp, err := a.access.Issue(user.ID, user.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	refreshTok, refreshExp, err := a.refresh.Mint(ctx, user.ID, nil, nil, req.Header().Get("User-Agent"))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userifacev1.LoginResponse{
		AccessToken:      accessTok,
		RefreshToken:     refreshTok,
		User:             toProto(&user),
		AccessExpiresAt:  accessExp.Unix(),
		RefreshExpiresAt: refreshExp.Unix(),
	}), nil
}

func (a *Auth) Refresh(
	ctx context.Context,
	req *connect.Request[userifacev1.RefreshRequest],
) (*connect.Response[userifacev1.RefreshResponse], error) {
	newRaw, newExp, userID, role, err := a.refresh.Rotate(ctx, req.Msg.RefreshToken, req.Header().Get("User-Agent"))
	if err != nil {
		return nil, err
	}

	accessTok, accessExp, err := a.access.Issue(userID, role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userifacev1.RefreshResponse{
		AccessToken:      accessTok,
		RefreshToken:     newRaw,
		AccessExpiresAt:  accessExp.Unix(),
		RefreshExpiresAt: newExp.Unix(),
	}), nil
}

func (a *Auth) Logout(
	ctx context.Context,
	req *connect.Request[userifacev1.LogoutRequest],
) (*connect.Response[userifacev1.LogoutResponse], error) {
	// Best-effort: unknown / already-revoked tokens succeed silently.
	if err := a.refresh.Revoke(ctx, req.Msg.RefreshToken); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&userifacev1.LogoutResponse{}), nil
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
