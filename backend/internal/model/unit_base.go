package model

import "time"

type UnitBase struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name      string    `gorm:"uniqueIndex;not null"`
	Active    bool      `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (UnitBase) TableName() string { return "unit_bases" }
