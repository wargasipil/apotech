package model

import "time"

type Prescription struct {
	ID          string    `gorm:"primaryKey;type:uuid"`
	RxNo        *string   `gorm:"uniqueIndex;column:rx_no"`
	CustomerID  string    `gorm:"not null;type:uuid;column:customer_id"`
	IssuerName  string    `gorm:"not null;column:issuer_name"`
	IssuedAt    time.Time `gorm:"not null;type:date;column:issued_at"`
	ExpiresAt   time.Time `gorm:"not null;type:date;column:expires_at"`
	Note        string    `gorm:"not null;default:''"`
	Status      string    `gorm:"not null;default:'ACTIVE'"`
	CreatedBy   string    `gorm:"not null;type:uuid;column:created_by"`
	BranchID    *string   `gorm:"type:uuid;column:branch_id"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Items []PrescriptionItem `gorm:"foreignKey:PrescriptionID"`
}

func (Prescription) TableName() string { return "prescriptions" }

type PrescriptionItem struct {
	ID                  string `gorm:"primaryKey;type:uuid"`
	PrescriptionID      string `gorm:"not null;type:uuid;column:prescription_id"`
	MedicineID          string `gorm:"not null;type:uuid;column:medicine_id"`
	PrescribedQty       int32  `gorm:"not null;column:prescribed_qty"`
	DispensedQty        int32  `gorm:"not null;default:0;column:dispensed_qty"`
	DosageInstructions  string `gorm:"not null;default:'';column:dosage_instructions"`
	Note                string `gorm:"not null;default:''"`
	CreatedAt           time.Time
}

func (PrescriptionItem) TableName() string { return "prescription_items" }

type RxCounter struct {
	Year    int `gorm:"primaryKey"`
	LastSeq int `gorm:"not null;default:0;column:last_seq"`
}

func (RxCounter) TableName() string { return "rx_no_counters" }
