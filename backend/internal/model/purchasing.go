package model

import "time"

type PurchaseOrder struct {
	ID           string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PoNo         *string    `gorm:"uniqueIndex;column:po_no"`
	SupplierID   string     `gorm:"not null;type:uuid;column:supplier_id"`
	Status       string     `gorm:"not null;default:'DRAFT'"`
	InvoiceNo    string     `gorm:"not null;default:'';column:invoice_no"`
	InvoiceDate  *time.Time `gorm:"type:date;column:invoice_date"`
	DueAt        *time.Time `gorm:"type:date;column:due_at"`
	Note         string     `gorm:"not null;default:''"`
	Subtotal     int64      `gorm:"not null;default:0;column:subtotal"`
	CartDiscount int64      `gorm:"not null;default:0;column:cart_discount"`
	PpnEnabled   bool       `gorm:"not null;default:false;column:ppn_enabled"`
	PpnRate      int32      `gorm:"not null;default:11;column:ppn_rate"` // percent (0-100); ignored when PpnEnabled=false
	PpnAmount    int64      `gorm:"not null;default:0;column:ppn_amount"`
	OrderedTotal int64      `gorm:"not null;default:0;column:ordered_total"` // = Subtotal − CartDiscount + PpnAmount
	PaidAmount   int64      `gorm:"not null;default:0;column:paid_amount"`
	CreatedBy    string     `gorm:"not null;type:uuid;column:created_by"`
	BranchID     *string    `gorm:"type:uuid;column:branch_id"`
	WarehouseID  string     `gorm:"not null;type:uuid;column:warehouse_id"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SentAt       *time.Time `gorm:"column:sent_at"`
	ClosedAt     *time.Time `gorm:"column:closed_at"`

	Items []PurchaseOrderItem `gorm:"foreignKey:PurchaseOrderID"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

type PurchaseOrderItem struct {
	ID              string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PurchaseOrderID string `gorm:"not null;type:uuid;column:purchase_order_id"`
	MedicineID      string `gorm:"not null;type:uuid;column:medicine_id"`
	OrderedQty      int32  `gorm:"not null;column:ordered_qty"`  // BASE units
	ReceivedQty     int32  `gorm:"not null;default:0;column:received_qty"` // BASE units
	UnitCostPrice   int64  `gorm:"not null;default:0;column:unit_cost_price"` // per BASE unit
	Subtotal        int64  `gorm:"not null;default:0"`
	// Purchasable unit the line was ordered in (display/entry metadata).
	MedicineUnitID *string `gorm:"type:uuid;column:medicine_unit_id"`
	UnitName       string  `gorm:"not null;default:'';column:unit_name"`
	UnitFactor     int64   `gorm:"not null;default:1;column:unit_factor"`
}

func (PurchaseOrderItem) TableName() string { return "purchase_order_items" }

type PurchaseReceipt struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ReceiptNo       *string   `gorm:"uniqueIndex;column:receipt_no"`
	PurchaseOrderID string    `gorm:"not null;type:uuid;column:purchase_order_id"`
	ReceivedAt      time.Time `gorm:"not null;type:date;column:received_at"`
	ReceivedBy      string    `gorm:"not null;type:uuid;column:received_by"`
	Note            string    `gorm:"not null;default:''"`
	InvoiceNo       string    `gorm:"not null;default:'';column:invoice_no"`
	CreatedAt       time.Time

	Items []PurchaseReceiptItem `gorm:"foreignKey:PurchaseReceiptID"`
}

func (PurchaseReceipt) TableName() string { return "purchase_receipts" }

type PurchaseReceiptItem struct {
	ID                  string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PurchaseReceiptID   string    `gorm:"not null;type:uuid;column:purchase_receipt_id"`
	PurchaseOrderItemID string    `gorm:"not null;type:uuid;column:purchase_order_item_id"`
	MedicineID          string    `gorm:"not null;type:uuid;column:medicine_id"`
	Qty                 int32     `gorm:"not null"` // BASE units
	UnitCostPrice       int64     `gorm:"not null;default:0;column:unit_cost_price"` // per BASE unit
	BatchNumber         string    `gorm:"not null;default:'';column:batch_number"`
	ExpiryDate          time.Time `gorm:"not null;type:date;column:expiry_date"`
	BatchID             *string   `gorm:"type:uuid;column:batch_id"`
	CreatedAt           time.Time
	// Purchasable unit the line was received in (display/entry metadata).
	MedicineUnitID *string `gorm:"type:uuid;column:medicine_unit_id"`
	UnitName       string  `gorm:"not null;default:'';column:unit_name"`
	UnitFactor     int64   `gorm:"not null;default:1;column:unit_factor"`
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
