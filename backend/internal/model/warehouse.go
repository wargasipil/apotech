package model

import "time"

// Warehouse (gudang) — the stock location concept that replaces Branch.
// Stock is partitioned per warehouse via stock_movements.warehouse_id.
type Warehouse struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Code      string `gorm:"uniqueIndex;not null"`
	Name      string `gorm:"not null"`
	Address   string `gorm:"not null;default:''"`
	Phone     string `gorm:"not null;default:''"`
	IsDefault bool   `gorm:"not null;default:false;column:is_default"`
	Active    bool   `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Warehouse) TableName() string { return "warehouses" }

type UserWarehouse struct {
	UserID      string `gorm:"primaryKey;type:uuid;column:user_id"`
	WarehouseID string `gorm:"primaryKey;type:uuid;column:warehouse_id"`
	IsDefault   bool   `gorm:"not null;default:false;column:is_default"`
	CreatedAt   time.Time
}

func (UserWarehouse) TableName() string { return "user_warehouses" }
