package model

import "time"

type Batch struct {
	ID          string  `gorm:"primaryKey;type:uuid"`
	MedicineID  string  `gorm:"not null;type:uuid;column:medicine_id"`
	SupplierID  *string `gorm:"type:uuid;column:supplier_id"`
	BatchNumber string  `gorm:"not null;default:'';column:batch_number"`
	ExpiryDate  time.Time `gorm:"not null;type:date;column:expiry_date"`
	CostPrice   int64   `gorm:"not null;default:0;column:cost_price"`
	ReceivedAt  time.Time `gorm:"not null;type:date;column:received_at"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Batch) TableName() string { return "batches" }
