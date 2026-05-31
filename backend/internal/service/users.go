package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	authifacev1 "github.com/apotech/backend/gen/auth_iface/v1"
	userifacev1 "github.com/apotech/backend/gen/user_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/config"
	"github.com/apotech/backend/internal/model"
)

const passwordResetTTL = 24 * time.Hour

const (
	roleOwner      = "OWNER"
	rolePharmacist = "PHARMACIST"
	roleCashier    = "CASHIER"
)

type Users struct {
	db *gorm.DB
}

func NewUsers(db *gorm.DB) *Users { return &Users{db: db} }

// grantDefaultWarehouse ensures the user has a membership for the global
// default warehouse. Idempotent — safe to call on every boot. The membership's
// is_default is set true only when the user has no existing default (so a
// per-user default explicitly set later via SetDefaultWarehouse is preserved).
//
// Closes the gap in migration 00019_warehouses.sql, which grants every
// EXISTING user access to MAIN at migration time — users created after that
// (bootstrap owner, CreateUser admin path) used to land with zero memberships.
func grantDefaultWarehouse(tx *gorm.DB, userID string) error {
	var defaultWHID string
	err := tx.Table("warehouses").Select("id").Where("is_default").Limit(1).Scan(&defaultWHID).Error
	if err != nil {
		return fmt.Errorf("lookup default warehouse: %w", err)
	}
	if defaultWHID == "" {
		// No default warehouse seeded; nothing to grant.
		return nil
	}
	// If the user already has any default-flagged membership, don't disrupt it
	// — grant the default warehouse non-defaulted.
	var hasDefault bool
	err = tx.Raw(`SELECT EXISTS (
		SELECT 1 FROM user_warehouses WHERE user_id = ? AND is_default
	)`, userID).Scan(&hasDefault).Error
	if err != nil {
		return fmt.Errorf("check existing default: %w", err)
	}
	// INSERT ... ON CONFLICT DO NOTHING — re-running on an already-granted
	// user is a true no-op (keeps whatever is_default that row already has).
	res := tx.Exec(`
		INSERT INTO user_warehouses (user_id, warehouse_id, is_default)
		VALUES (?, ?, ?)
		ON CONFLICT (user_id, warehouse_id) DO NOTHING
	`, userID, defaultWHID, !hasDefault)
	if res.Error != nil {
		return fmt.Errorf("grant default warehouse: %w", res.Error)
	}
	return nil
}

// EnsureBootstrapOwner upserts the owner described in config.Bootstrap.
// If owner_email is empty, it's a no-op (with a warning to the caller).
func (u *Users) EnsureBootstrapOwner(ctx context.Context, b config.Bootstrap) error {
	if b.OwnerEmail == "" {
		fmt.Println("bootstrap: owner_email empty; skipping owner ensure")
		return nil
	}
	if b.OwnerPassword == "" {
		return fmt.Errorf("bootstrap: owner_password must be set when owner_email is set")
	}

	hash, err := auth.HashPassword(b.OwnerPassword)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}

	var existing model.User
	err = u.db.WithContext(ctx).Where("email = ?", b.OwnerEmail).First(&existing).Error
	var ownerID string
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		newUser := model.User{
			Email:        b.OwnerEmail,
			Name:         "Owner",
			PasswordHash: hash,
			Role:         roleOwner,
			Active:       true,
		}
		if err := u.db.WithContext(ctx).Create(&newUser).Error; err != nil {
			return fmt.Errorf("create bootstrap owner: %w", err)
		}
		ownerID = newUser.ID
		fmt.Printf("bootstrap: created owner %s\n", b.OwnerEmail)
	case err != nil:
		return fmt.Errorf("lookup bootstrap owner: %w", err)
	default:
		if err := u.db.WithContext(ctx).Model(&existing).Updates(map[string]any{
			"password_hash": hash,
			"role":          roleOwner,
			"active":        true,
		}).Error; err != nil {
			return fmt.Errorf("update bootstrap owner: %w", err)
		}
		ownerID = existing.ID
		fmt.Printf("bootstrap: ensured owner %s\n", b.OwnerEmail)
	}
	// Idempotent: grant the owner access to the default warehouse if no
	// existing membership for it. Heals owners created on a fresh DB where
	// migration 00019's at-migration-time grant had no users to grant.
	if err := grantDefaultWarehouse(u.db.WithContext(ctx), ownerID); err != nil {
		return fmt.Errorf("bootstrap owner default warehouse grant: %w", err)
	}
	return nil
}

func (u *Users) ListUsers(
	ctx context.Context,
	_ *connect.Request[userifacev1.ListUsersRequest],
) (*connect.Response[userifacev1.ListUsersResponse], error) {
	var rows []model.User
	if err := u.db.WithContext(ctx).Order("created_at").Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*userifacev1.User, 0, len(rows))
	for _, r := range rows {
		out = append(out, toProto(&r))
	}
	return connect.NewResponse(&userifacev1.ListUsersResponse{Users: out}), nil
}

// ResolveUsers returns minimal display refs for a set of ids — the
// batch-resolve-by-IDs pattern (mirrors ResolveCustomers). Used by analytics
// pages so the metric handler can return user_ids only.
func (u *Users) ResolveUsers(
	ctx context.Context,
	req *connect.Request[userifacev1.ResolveUsersRequest],
) (*connect.Response[userifacev1.ResolveUsersResponse], error) {
	ids := dedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&userifacev1.ResolveUsersResponse{}), nil
	}
	type row struct {
		ID    string `gorm:"column:id"`
		Name  string `gorm:"column:name"`
		Email string `gorm:"column:email"`
	}
	var rows []row
	if err := u.db.WithContext(ctx).
		Model(&model.User{}).
		Select("id, name, email").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*userifacev1.UserRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &userifacev1.UserRef{Id: r.ID, Name: r.Name, Email: r.Email})
	}
	return connect.NewResponse(&userifacev1.ResolveUsersResponse{Users: out}), nil
}

// SearchUsers — server-side fuzzy search for the warehouse-detail "Add user"
// picker. Mirrors SearchCustomers / SearchSuppliers. ILIKE on email + name.
func (u *Users) SearchUsers(
	ctx context.Context,
	req *connect.Request[userifacev1.SearchUsersRequest],
) (*connect.Response[userifacev1.SearchUsersResponse], error) {
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := strings.TrimSpace(req.Msg.Query)
	type row struct {
		ID    string `gorm:"column:id"`
		Name  string `gorm:"column:name"`
		Email string `gorm:"column:email"`
	}
	tx := u.db.WithContext(ctx).Model(&model.User{}).Select("id, name, email")
	if q != "" {
		like := "%" + q + "%"
		tx = tx.Where("email ILIKE ? OR name ILIKE ?", like, like)
	}
	var rows []row
	if err := tx.Order("email ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*userifacev1.UserRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &userifacev1.UserRef{Id: r.ID, Name: r.Name, Email: r.Email})
	}
	return connect.NewResponse(&userifacev1.SearchUsersResponse{Users: out}), nil
}

func (u *Users) CreateUser(
	ctx context.Context,
	req *connect.Request[userifacev1.CreateUserRequest],
) (*connect.Response[userifacev1.CreateUserResponse], error) {
	m := req.Msg
	email := strings.TrimSpace(m.Email)
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email required"))
	}
	if len(m.Password) < 8 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("password must be at least 8 characters"))
	}
	roleStr, err := roleFromProto(m.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	hash, err := auth.HashPassword(m.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	user := model.User{
		Email:        email,
		Name:         strings.TrimSpace(m.Name),
		PasswordHash: hash,
		Role:         roleStr,
		Active:       true,
	}
	if err := u.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("create user: %w", err))
	}
	// Grant the new user access to the default warehouse (their first usable
	// location). Owners can later grant access to additional warehouses via
	// the /warehouses admin UI.
	if err := grantDefaultWarehouse(u.db.WithContext(ctx), user.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("grant default warehouse: %w", err))
	}

	return connect.NewResponse(&userifacev1.CreateUserResponse{User: toProto(&user)}), nil
}

func (u *Users) UpdateUserRole(
	ctx context.Context,
	req *connect.Request[userifacev1.UpdateUserRoleRequest],
) (*connect.Response[userifacev1.UpdateUserRoleResponse], error) {
	roleStr, err := roleFromProto(req.Msg.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	user, err := u.loadByID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, err
	}
	if err := u.db.WithContext(ctx).Model(user).Update("role", roleStr).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	user.Role = roleStr
	return connect.NewResponse(&userifacev1.UpdateUserRoleResponse{User: toProto(user)}), nil
}

func (u *Users) SetUserActive(
	ctx context.Context,
	req *connect.Request[userifacev1.SetUserActiveRequest],
) (*connect.Response[userifacev1.SetUserActiveResponse], error) {
	user, err := u.loadByID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, err
	}
	if err := u.db.WithContext(ctx).Model(user).Update("active", req.Msg.Active).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	user.Active = req.Msg.Active
	return connect.NewResponse(&userifacev1.SetUserActiveResponse{User: toProto(user)}), nil
}

func (u *Users) ChangePassword(
	ctx context.Context,
	req *connect.Request[userifacev1.ChangePasswordRequest],
) (*connect.Response[userifacev1.ChangePasswordResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.Msg.NewPassword) < 8 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("new_password must be at least 8 characters"))
	}

	targetID := req.Msg.UserId
	isSelf := targetID == "" || targetID == caller.UserID
	if !isSelf && caller.Role != roleOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only OWNER can change another user's password"))
	}
	if isSelf {
		targetID = caller.UserID
	}

	target, err := u.loadByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if isSelf {
		if err := auth.VerifyPassword(target.PasswordHash, req.Msg.OldPassword); err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("old_password incorrect"))
		}
	}

	hash, err := auth.HashPassword(req.Msg.NewPassword)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := u.db.WithContext(ctx).Model(target).Update("password_hash", hash).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&userifacev1.ChangePasswordResponse{}), nil
}

func (u *Users) loadByID(ctx context.Context, id string) (*model.User, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id required"))
	}
	var user model.User
	err := u.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &user, nil
}

func toProto(u *model.User) *userifacev1.User {
	return &userifacev1.User{
		Id:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      roleToProto(u.Role),
		Active:    u.Active,
		CreatedAt: u.CreatedAt.Unix(),
	}
}

func roleFromProto(r authifacev1.Role) (string, error) {
	switch r {
	case authifacev1.Role_ROLE_OWNER:
		return roleOwner, nil
	case authifacev1.Role_ROLE_PHARMACIST:
		return rolePharmacist, nil
	case authifacev1.Role_ROLE_CASHIER:
		return roleCashier, nil
	default:
		return "", errors.New("role required")
	}
}

func roleToProto(s string) authifacev1.Role {
	switch s {
	case roleOwner:
		return authifacev1.Role_ROLE_OWNER
	case rolePharmacist:
		return authifacev1.Role_ROLE_PHARMACIST
	case roleCashier:
		return authifacev1.Role_ROLE_CASHIER
	default:
		return authifacev1.Role_ROLE_UNSPECIFIED
	}
}

// ---------- Password reset ----------

func (u *Users) IssuePasswordResetToken(
	ctx context.Context,
	req *connect.Request[userifacev1.IssuePasswordResetTokenRequest],
) (*connect.Response[userifacev1.IssuePasswordResetTokenResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.UserId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id required"))
	}
	var target model.User
	if err := u.db.WithContext(ctx).Where("id = ?", req.Msg.UserId).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	raw, hash, err := generateResetToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	exp := time.Now().Add(passwordResetTTL)
	row := model.PasswordResetToken{
		UserID:    target.ID,
		TokenHash: hash,
		IssuedBy:  caller.UserID,
		ExpiresAt: exp,
	}
	if err := u.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&userifacev1.IssuePasswordResetTokenResponse{
		Token:     raw,
		ExpiresAt: exp.Unix(),
	}), nil
}

func (u *Users) RedeemPasswordResetToken(
	ctx context.Context,
	req *connect.Request[userifacev1.RedeemPasswordResetTokenRequest],
) (*connect.Response[userifacev1.RedeemPasswordResetTokenResponse], error) {
	if strings.TrimSpace(req.Msg.Token) == "" || len(req.Msg.NewPassword) < 8 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("token and new_password (min 8 chars) required"))
	}
	hash := hashResetToken(req.Msg.Token)
	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.PasswordResetToken
		if err := tx.Where("token_hash = ?", hash).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if row.UsedAt != nil {
			return connect.NewError(connect.CodeUnauthenticated, errors.New("token already used"))
		}
		if time.Now().After(row.ExpiresAt) {
			return connect.NewError(connect.CodeUnauthenticated, errors.New("token expired"))
		}
		newHash, err := auth.HashPassword(req.Msg.NewPassword)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		now := time.Now()
		if err := tx.Model(&model.User{}).Where("id = ?", row.UserID).
			Update("password_hash", newHash).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&row).Update("used_at", now).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	return connect.NewResponse(&userifacev1.RedeemPasswordResetTokenResponse{}), nil
}

func generateResetToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	return raw, hashResetToken(raw), nil
}

func hashResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
