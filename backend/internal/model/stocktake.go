package model

import "time"

type StocktakeSession struct {
	ID          string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name        string     `gorm:"not null;default:''"`
	Status      string     `gorm:"not null;default:'DRAFT'"`
	BranchID    *string    `gorm:"type:uuid;column:branch_id"` // deprecated; superseded by warehouse_id
	WarehouseID *string    `gorm:"type:uuid;column:warehouse_id"`
	CreatedBy   string     `gorm:"not null;type:uuid;column:created_by"`
	CreatedAt   time.Time
	CompletedAt *time.Time `gorm:"column:completed_at"`
	VoidedAt    *time.Time `gorm:"column:voided_at"`

	Lines []StocktakeLine `gorm:"foreignKey:SessionID"`
}

func (StocktakeSession) TableName() string { return "stocktake_sessions" }

type StocktakeLine struct {
	ID              string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	SessionID       string     `gorm:"not null;type:uuid;column:session_id"`
	BatchID         string     `gorm:"not null;type:uuid;column:batch_id"`
	ExpectedQty     int32      `gorm:"not null;column:expected_qty"`
	CountedQty      *int32     `gorm:"column:counted_qty"`
	Disposition     string     `gorm:"not null;default:'ADJUSTMENT'"`
	WriteOffKind    *string    `gorm:"column:write_off_kind"`
	DispositionNote string     `gorm:"not null;default:'';column:disposition_note"`
	CountedAt       *time.Time `gorm:"column:counted_at"`
	CountedBy       *string    `gorm:"type:uuid;column:counted_by"`
	CreatedAt       time.Time
}

func (StocktakeLine) TableName() string { return "stocktake_lines" }
