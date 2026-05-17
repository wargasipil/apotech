package model

import "time"

type AuditEntry struct {
	ID         int64   `gorm:"primaryKey;autoIncrement"`
	UserID     *string `gorm:"type:uuid;column:user_id"`
	Role       string  `gorm:"not null;default:''"`
	BranchID   *string `gorm:"type:uuid;column:branch_id"`
	Procedure  string  `gorm:"not null"`
	OK         bool    `gorm:"not null;column:ok"`
	Code       string  `gorm:"not null;default:''"`
	Message    string  `gorm:"not null;default:''"`
	IP         string  `gorm:"not null;default:''"`
	UserAgent  string  `gorm:"not null;default:'';column:user_agent"`
	DurationMS int32   `gorm:"not null;default:0;column:duration_ms"`
	CreatedAt  time.Time
}

func (AuditEntry) TableName() string { return "audit_log" }
