package model

import "time"

type Medicine struct {
	ID                   string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	SKU                  string `gorm:"uniqueIndex;not null;column:sku"`
	Name                 string `gorm:"not null"`
	Manufacturer         string `gorm:"not null;default:''"`
	Unit                 string `gorm:"not null"`
	UnitPrice            int64  `gorm:"not null;column:unit_price"`
	PrescriptionRequired bool   `gorm:"not null;default:false;column:prescription_required"`
	Active               bool   `gorm:"not null;default:true"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (Medicine) TableName() string { return "medicines" }
