package model

import "time"

// MedicineUnitPrice is the per-unit sell-price history, mirroring MedicinePrice
// but keyed by a medicine_unit. Exactly one open row (EffectiveTo == nil) per
// unit. ChangedBy is nullable so the migration backfill can seed a baseline.
type MedicineUnitPrice struct {
	ID             string     `gorm:"primaryKey;type:uuid"`
	MedicineUnitID string     `gorm:"not null;type:uuid;column:medicine_unit_id"`
	UnitSellPrice  int64      `gorm:"not null;column:unit_sell_price"`
	EffectiveFrom  time.Time  `gorm:"not null;column:effective_from"`
	EffectiveTo    *time.Time `gorm:"column:effective_to"` // NULL = current/open
	ChangedBy      *string    `gorm:"type:uuid;column:changed_by"`
	CreatedAt      time.Time
}

func (MedicineUnitPrice) TableName() string { return "medicine_unit_prices" }
