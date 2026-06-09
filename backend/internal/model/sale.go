package model

import "time"

type Sale struct {
	ID             string     `gorm:"primaryKey;type:uuid"`
	SaleNo         *string    `gorm:"uniqueIndex;column:sale_no"`
	CustomerID     *string    `gorm:"type:uuid;column:customer_id"`
	CashierUserID  string     `gorm:"not null;type:uuid;column:cashier_user_id"`
	PaymentSource  *string    `gorm:"column:payment_source"`
	TaxInvoiceNo   *string    `gorm:"column:tax_invoice_no"`
	Subtotal       int64      `gorm:"not null;default:0"`
	CartDiscount   int64      `gorm:"not null;default:0;column:cart_discount"`
	Total          int64      `gorm:"not null;default:0"`
	PaidAmount     int64      `gorm:"not null;default:0;column:paid_amount"`
	Status         string     `gorm:"not null;default:'DRAFT'"`
	BranchID       *string    `gorm:"type:uuid;column:branch_id"` // deprecated; superseded by warehouse_id
	WarehouseID    *string    `gorm:"type:uuid;column:warehouse_id"`
	PrescriptionID *string    `gorm:"type:uuid;column:prescription_id"`
	TaxInvoiceCode *string    `gorm:"column:tax_invoice_code"`
	TaxInvoiceDPP  int64      `gorm:"not null;default:0;column:tax_invoice_dpp"`
	TaxInvoicePPN  int64      `gorm:"not null;default:0;column:tax_invoice_ppn"`
	TaxInvoiceIssuedAt *time.Time `gorm:"column:tax_invoice_issued_at"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time `gorm:"column:completed_at"`
	CancelledAt    *time.Time `gorm:"column:cancelled_at"` // when a COMPLETED sale was cancelled via VoidSale

	Items []SaleItem `gorm:"foreignKey:SaleID"`
}

func (Sale) TableName() string { return "sales" }

type SaleItem struct {
	ID                string  `gorm:"primaryKey;type:uuid"`
	SaleID            string  `gorm:"not null;type:uuid;column:sale_id"`
	MedicineID        string  `gorm:"not null;type:uuid;column:medicine_id"`
	BatchID           *string `gorm:"type:uuid;column:batch_id"`
	Qty               int32   `gorm:"not null"` // qty in the selling unit
	UnitPriceSnapshot int64   `gorm:"not null;default:0;column:unit_price_snapshot"`
	LineDiscount      int64   `gorm:"not null;default:0;column:line_discount"`
	LineTotal         int64   `gorm:"not null;default:0;column:line_total"`
	BranchID          *string `gorm:"type:uuid;column:branch_id"`
	MedicineUnitID    *string `gorm:"type:uuid;column:medicine_unit_id"`
	UnitName          string  `gorm:"not null;default:'';column:unit_name"`
	UnitFactor        int64   `gorm:"not null;default:1;column:unit_factor"`
	BaseQty           int32   `gorm:"not null;default:0;column:base_qty"`
	CreatedAt         time.Time
}

func (SaleItem) TableName() string { return "sale_items" }

type SaleNoCounter struct {
	Year    int `gorm:"primaryKey"`
	LastSeq int `gorm:"not null;default:0;column:last_seq"`
}

func (SaleNoCounter) TableName() string { return "sale_no_counters" }
