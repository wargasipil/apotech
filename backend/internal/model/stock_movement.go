package model

import "time"

type StockMovement struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	BatchID         string    `gorm:"not null;type:uuid;column:batch_id"`
	Qty             int32     `gorm:"not null"`
	Type            string    `gorm:"not null"`
	Reason          string    `gorm:"not null;default:''"`
	UserID          string    `gorm:"not null;type:uuid;column:user_id"`
	SaleItemID      *string   `gorm:"type:uuid;column:sale_item_id"`
	BranchID        *string   `gorm:"type:uuid;column:branch_id"` // deprecated; superseded by warehouse_id
	WarehouseID     string    `gorm:"not null;type:uuid;column:warehouse_id"`
	TransferID      *string   `gorm:"type:uuid;column:transfer_id"`
	StocktakeLineID *string   `gorm:"type:uuid;column:stocktake_line_id"`
	WriteOffKind    *string   `gorm:"column:write_off_kind"`
	CreatedAt       time.Time
}

func (StockMovement) TableName() string { return "stock_movements" }
