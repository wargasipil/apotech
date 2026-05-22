package model

import "time"

// StockTransfer is the header for a stock movement between two warehouses.
// Each transferred batch produces a TRANSFER_OUT (source, negative) and a
// TRANSFER_IN (destination, positive) stock_movements row linked via transfer_id.
type StockTransfer struct {
	ID              string  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	TransferNo      *string `gorm:"uniqueIndex;column:transfer_no"` // TRF-YYYY-NNNN
	FromWarehouseID string  `gorm:"not null;type:uuid;column:from_warehouse_id"`
	ToWarehouseID   string  `gorm:"not null;type:uuid;column:to_warehouse_id"`
	Note            string  `gorm:"not null;default:''"`
	CreatedBy       string  `gorm:"not null;type:uuid;column:created_by"`
	CreatedAt       time.Time
}

func (StockTransfer) TableName() string { return "stock_transfers" }

// TransferCounter assigns per-year sequential transfer numbers.
type TransferCounter struct {
	Year    int `gorm:"primaryKey"`
	LastSeq int `gorm:"not null;default:0;column:last_seq"`
}

func (TransferCounter) TableName() string { return "transfer_no_counters" }
