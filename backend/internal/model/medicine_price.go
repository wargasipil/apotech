package model

import "time"

type MedicinePrice struct {
	ID            string     `gorm:"primaryKey;type:uuid"`
	MedicineID    string     `gorm:"not null;type:uuid;column:medicine_id"`
	UnitPrice     int64      `gorm:"not null;column:unit_price"`
	EffectiveFrom time.Time  `gorm:"not null;column:effective_from"`
	EffectiveTo   *time.Time `gorm:"column:effective_to"` // NULL = current/open
	ChangedBy     string     `gorm:"not null;type:uuid;column:changed_by"`
	CreatedAt     time.Time
}

func (MedicinePrice) TableName() string { return "medicine_prices" }
