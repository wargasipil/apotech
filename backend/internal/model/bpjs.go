package model

import "time"

type BpjsClaim struct {
	ID          string     `gorm:"primaryKey;type:uuid"`
	SaleID      string     `gorm:"not null;type:uuid;column:sale_id"`
	CustomerID  string     `gorm:"not null;type:uuid;column:customer_id"`
	BPJSNo      string     `gorm:"not null;column:bpjs_no"`
	Status      string     `gorm:"not null;default:'DRAFT'"`
	Amount      int64      `gorm:"not null;default:0"`
	ExternalRef string     `gorm:"not null;default:'';column:external_ref"`
	Note        string     `gorm:"not null;default:''"`
	SubmittedAt *time.Time `gorm:"column:submitted_at"`
	ResolvedAt  *time.Time `gorm:"column:resolved_at"`
	CreatedBy   string     `gorm:"not null;type:uuid;column:created_by"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (BpjsClaim) TableName() string { return "bpjs_claims" }
