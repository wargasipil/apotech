package model

import "time"

type PurchaseOrder struct {
	ID            string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PoNo          *string    `gorm:"uniqueIndex;column:po_no"`
	SupplierID    string     `gorm:"not null;type:uuid;column:supplier_id"`
	Status        string     `gorm:"not null;default:'DRAFT'"`
	ExpectedAt    *time.Time `gorm:"type:date;column:expected_at"`
	Note          string     `gorm:"not null;default:''"`
	OrderedTotal  int64      `gorm:"not null;default:0;column:ordered_total"`
	PaidAmount    int64      `gorm:"not null;default:0;column:paid_amount"`
	CreatedBy     string     `gorm:"not null;type:uuid;column:created_by"`
	BranchID      *string    `gorm:"type:uuid;column:branch_id"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SentAt        *time.Time `gorm:"column:sent_at"`
	ClosedAt      *time.Time `gorm:"column:closed_at"`

	Items []PurchaseOrderItem `gorm:"foreignKey:PurchaseOrderID"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

type PurchaseOrderItem struct {
	ID              string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PurchaseOrderID string `gorm:"not null;type:uuid;column:purchase_order_id"`
	MedicineID      string `gorm:"not null;type:uuid;column:medicine_id"`
	OrderedQty      int32  `gorm:"not null;column:ordered_qty"`
	ReceivedQty     int32  `gorm:"not null;default:0;column:received_qty"`
	UnitCostPrice   int64  `gorm:"not null;default:0;column:unit_cost_price"`
	Subtotal        int64  `gorm:"not null;default:0"`
}

func (PurchaseOrderItem) TableName() string { return "purchase_order_items" }

type PurchaseReceipt struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ReceiptNo       *string   `gorm:"uniqueIndex;column:receipt_no"`
	PurchaseOrderID string    `gorm:"not null;type:uuid;column:purchase_order_id"`
	ReceivedAt      time.Time `gorm:"not null;type:date;column:received_at"`
	ReceivedBy      string    `gorm:"not null;type:uuid;column:received_by"`
	Note            string    `gorm:"not null;default:''"`
	CreatedAt       time.Time

	Items []PurchaseReceiptItem `gorm:"foreignKey:PurchaseReceiptID"`
}

func (PurchaseReceipt) TableName() string { return "purchase_receipts" }

type PurchaseReceiptItem struct {
	ID                  string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PurchaseReceiptID   string    `gorm:"not null;type:uuid;column:purchase_receipt_id"`
	PurchaseOrderItemID string    `gorm:"not null;type:uuid;column:purchase_order_item_id"`
	MedicineID          string    `gorm:"not null;type:uuid;column:medicine_id"`
	Qty                 int32     `gorm:"not null"`
	UnitCostPrice       int64     `gorm:"not null;default:0;column:unit_cost_price"`
	BatchNumber         string    `gorm:"not null;default:'';column:batch_number"`
	ExpiryDate          time.Time `gorm:"not null;type:date;column:expiry_date"`
	BatchID             *string   `gorm:"type:uuid;column:batch_id"`
	CreatedAt           time.Time
}

func (PurchaseReceiptItem) TableName() string { return "purchase_receipt_items" }

type POCounter struct {
	Year    int `gorm:"primaryKey"`
	LastSeq int `gorm:"not null;default:0;column:last_seq"`
}

func (POCounter) TableName() string { return "po_no_counters" }

type RcvCounter struct {
	Year    int `gorm:"primaryKey"`
	LastSeq int `gorm:"not null;default:0;column:last_seq"`
}

func (RcvCounter) TableName() string { return "rcv_no_counters" }
