package model

import "time"

type User struct {
	ID           string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Email        string `gorm:"uniqueIndex;not null"`
	Name         string `gorm:"not null;default:''"`
	PasswordHash string `gorm:"not null;column:password_hash"`
	Role         string `gorm:"not null;index"`
	Active       bool   `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "users" }
