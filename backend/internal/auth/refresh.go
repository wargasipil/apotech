package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/apotech/backend/internal/model"
)

// RefreshIssuer mints, rotates, and revokes opaque refresh tokens.
//   - The raw token is 32 random bytes hex-encoded (256 bits of entropy).
//   - Only its SHA-256 hash is stored in the DB.
//   - Every Rotate call invalidates the presented token and mints a new one
//     in the same family. Replaying a revoked-but-unexpired token revokes
//     the entire family (reuse detection).
type RefreshIssuer struct {
	DB  *gorm.DB
	TTL time.Duration
}

// hashToken returns the canonical sha256 hex of a raw refresh token.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateRaw() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// Mint creates a brand-new refresh token (new family if parent is nil, else
// a child in the same family). Returns the raw token and its expiry.
func (r *RefreshIssuer) Mint(
	ctx context.Context,
	userID string,
	familyID *string,
	parentID *string,
	userAgent string,
) (string, time.Time, error) {
	raw, err := generateRaw()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate refresh: %w", err)
	}
	exp := time.Now().Add(r.TTL)

	row := &model.RefreshToken{
		UserID:    userID,
		TokenHash: hashToken(raw),
		ParentID:  parentID,
		ExpiresAt: exp,
		UserAgent: userAgent,
	}
	if familyID != nil {
		row.FamilyID = *familyID
	}
	// If no family yet, let the DB default-insert generate one for `id`, then
	// set family_id = id so the family root references itself. We do this in
	// two steps inside a tx for atomicity.
	tx := r.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return "", time.Time{}, tx.Error
	}
	if familyID == nil {
		// Insert with a placeholder family_id (gen_random_uuid() default would
		// give us a fresh uuid, but we want family_id == id for the root).
		// Easiest: generate an id ourselves so we can set family_id at the
		// same time.
		row.FamilyID = "" // tmp; we'll assign after we know the id
	}
	// GORM with no explicit ID lets Postgres generate one. We need the id
	// to use as family_id when this is the root. Use Returning via Create.
	if err := tx.Create(row).Error; err != nil {
		tx.Rollback()
		return "", time.Time{}, fmt.Errorf("create refresh: %w", err)
	}
	if familyID == nil {
		if err := tx.Model(row).Update("family_id", row.ID).Error; err != nil {
			tx.Rollback()
			return "", time.Time{}, fmt.Errorf("set family_id: %w", err)
		}
	}
	if err := tx.Commit().Error; err != nil {
		return "", time.Time{}, err
	}
	return raw, exp, nil
}

// Rotate consumes a raw refresh token and returns a freshly minted one with
// the same family. On reuse (a revoked-but-unexpired token), the entire
// family is revoked and CodeUnauthenticated is returned.
func (r *RefreshIssuer) Rotate(
	ctx context.Context,
	raw string,
	userAgent string,
) (newRaw string, newExp time.Time, userID string, role string, err error) {
	if raw == "" {
		err = connect.NewError(connect.CodeUnauthenticated, errors.New("refresh_token required"))
		return
	}
	hash := hashToken(raw)

	var row model.RefreshToken
	err = r.DB.WithContext(ctx).Where("token_hash = ?", hash).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = connect.NewError(connect.CodeUnauthenticated, errors.New("invalid refresh_token"))
		return
	}
	if err != nil {
		err = connect.NewError(connect.CodeInternal, err)
		return
	}

	now := time.Now()
	if row.ExpiresAt.Before(now) {
		err = connect.NewError(connect.CodeUnauthenticated, errors.New("refresh_token expired"))
		return
	}

	// Reuse detection: a revoked-but-unexpired token is being replayed. Burn
	// the entire family and refuse.
	if row.RevokedAt != nil {
		_ = r.DB.WithContext(ctx).
			Model(&model.RefreshToken{}).
			Where("family_id = ? AND revoked_at IS NULL", row.FamilyID).
			Update("revoked_at", now).Error
		err = connect.NewError(connect.CodeUnauthenticated, errors.New("refresh_token reuse detected"))
		return
	}

	// Load the user (we need the current role for the access-token claims).
	var user model.User
	if uerr := r.DB.WithContext(ctx).Where("id = ?", row.UserID).First(&user).Error; uerr != nil {
		err = connect.NewError(connect.CodeInternal, uerr)
		return
	}
	if !user.Active {
		// User was disabled — revoke the family proactively.
		_ = r.DB.WithContext(ctx).
			Model(&model.RefreshToken{}).
			Where("family_id = ? AND revoked_at IS NULL", row.FamilyID).
			Update("revoked_at", now).Error
		err = connect.NewError(connect.CodePermissionDenied, errors.New("account disabled"))
		return
	}

	// Rotate: revoke the presented token, mint a child in the same family.
	tx := r.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		err = connect.NewError(connect.CodeInternal, tx.Error)
		return
	}
	if uerr := tx.Model(&row).Update("revoked_at", now).Error; uerr != nil {
		tx.Rollback()
		err = connect.NewError(connect.CodeInternal, uerr)
		return
	}
	// Mint within the same tx by inserting directly (we can't call Mint
	// because it opens its own tx).
	newRaw, gerr := generateRaw()
	if gerr != nil {
		tx.Rollback()
		err = connect.NewError(connect.CodeInternal, gerr)
		return
	}
	newExp = now.Add(r.TTL)
	parentID := row.ID
	familyID := row.FamilyID
	child := &model.RefreshToken{
		UserID:    row.UserID,
		TokenHash: hashToken(newRaw),
		FamilyID:  familyID,
		ParentID:  &parentID,
		ExpiresAt: newExp,
		UserAgent: userAgent,
	}
	if cerr := tx.Create(child).Error; cerr != nil {
		tx.Rollback()
		err = connect.NewError(connect.CodeInternal, cerr)
		return
	}
	if cerr := tx.Commit().Error; cerr != nil {
		err = connect.NewError(connect.CodeInternal, cerr)
		return
	}

	userID = user.ID
	role = user.Role
	return
}

// Revoke marks the entire family of the given raw token revoked. Idempotent
// when the token is unknown (logout twice = no error).
func (r *RefreshIssuer) Revoke(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	hash := hashToken(raw)
	var row model.RefreshToken
	err := r.DB.WithContext(ctx).Where("token_hash = ?", hash).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return r.DB.WithContext(ctx).
		Model(&model.RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL", row.FamilyID).
		Update("revoked_at", time.Now()).Error
}
