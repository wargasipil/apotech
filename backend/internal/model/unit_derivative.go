package model

import "time"

type UnitDerivative struct {
	ID         string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	BaseUnitID string    `gorm:"not null;type:uuid;column:base_unit_id;index"`
	Name       string    `gorm:"not null"`
	Factor     int64     `gorm:"not null"` // base units per 1 of this derivative; CHECK factor > 1
	SortOrder  int32     `gorm:"not null;default:0;column:sort_order"`
	Active     bool      `gorm:"not null;default:true"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (UnitDerivative) TableName() string { return "unit_derivatives" }
