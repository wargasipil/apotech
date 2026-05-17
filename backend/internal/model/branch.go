package model

import "time"

type Branch struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Code      string `gorm:"uniqueIndex;not null"`
	Name      string `gorm:"not null"`
	Address   string `gorm:"not null;default:''"`
	Phone     string `gorm:"not null;default:''"`
	Active    bool   `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Branch) TableName() string { return "branches" }

type UserBranch struct {
	UserID    string `gorm:"primaryKey;type:uuid;column:user_id"`
	BranchID  string `gorm:"primaryKey;type:uuid;column:branch_id"`
	IsDefault bool   `gorm:"not null;default:false;column:is_default"`
	CreatedAt time.Time
}

func (UserBranch) TableName() string { return "user_branches" }
