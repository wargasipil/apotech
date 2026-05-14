package model

import "time"

type RefreshToken struct {
	ID         string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	UserID     string     `gorm:"not null;type:uuid;column:user_id"`
	TokenHash  string     `gorm:"uniqueIndex;not null;column:token_hash"`
	FamilyID   string     `gorm:"not null;type:uuid;column:family_id"`
	ParentID   *string    `gorm:"type:uuid;column:parent_id"`
	ExpiresAt  time.Time  `gorm:"not null;column:expires_at"`
	RevokedAt  *time.Time `gorm:"column:revoked_at"`
	UserAgent  string     `gorm:"not null;default:'';column:user_agent"`
	CreatedAt  time.Time
}

func (RefreshToken) TableName() string { return "refresh_tokens" }
