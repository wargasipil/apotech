package model

import "time"

type PasswordResetToken struct {
	ID        string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	UserID    string     `gorm:"not null;type:uuid;column:user_id"`
	TokenHash string     `gorm:"uniqueIndex;not null;column:token_hash"`
	IssuedBy  string     `gorm:"not null;type:uuid;column:issued_by"`
	ExpiresAt time.Time  `gorm:"not null;column:expires_at"`
	UsedAt    *time.Time `gorm:"column:used_at"`
	CreatedAt time.Time
}

func (PasswordResetToken) TableName() string { return "password_reset_tokens" }
