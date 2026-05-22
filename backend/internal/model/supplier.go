package model

import "time"

type Supplier struct {
	ID           string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Code         string `gorm:"uniqueIndex;not null"`
	Name         string `gorm:"not null"`
	ContactEmail string `gorm:"column:contact_email"`
	Phone        string
	Active       bool `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Supplier) TableName() string { return "suppliers" }
