package model

import "time"

type NsfpEntry struct {
	ID         string     `gorm:"primaryKey;type:uuid"`
	Code       string     `gorm:"uniqueIndex;not null"`
	FiscalYear int        `gorm:"not null;column:fiscal_year"`
	ImportedBy string     `gorm:"not null;type:uuid;column:imported_by"`
	ImportedAt time.Time  `gorm:"not null;column:imported_at"`
	UsedAt     *time.Time `gorm:"column:used_at"`
	SaleID     *string    `gorm:"type:uuid;column:sale_id"`
}

func (NsfpEntry) TableName() string { return "nsfp_pool" }
