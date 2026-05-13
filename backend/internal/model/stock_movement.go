package model

import "time"

type StockMovement struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	BatchID   string    `gorm:"not null;type:uuid;column:batch_id"`
	Qty       int32     `gorm:"not null"`
	Type      string    `gorm:"not null"`
	Reason    string    `gorm:"not null;default:''"`
	UserID    string    `gorm:"not null;type:uuid;column:user_id"`
	CreatedAt time.Time
}

func (StockMovement) TableName() string { return "stock_movements" }
