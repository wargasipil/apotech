package service

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/apotech/backend/gen/purchasing_iface/v1"
	"github.com/apotech/backend/internal/model"
)

type PurchasePayments struct {
	db *gorm.DB
}

func NewPurchasePayments(db *gorm.DB) *PurchasePayments { return &PurchasePayments{db: db} }

func (p *PurchasePayments) PayPurchase(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.PayPurchaseRequest],
) (*connect.Response[purchasingifacev1.PayPurchaseResponse], error) {
	if req.Msg.PurchaseOrderId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("purchase_order_id required"))
	}
	if req.Msg.Amount <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("amount must be > 0"))
	}

	var paid, outstanding int64

	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ordersSvc := &PurchaseOrders{db: tx}
		po, err := ordersSvc.lockByID(tx, req.Msg.PurchaseOrderId)
		if err != nil {
			return err
		}
		if po.Status == poStatusVoided {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot pay a voided PO"))
		}
		newPaid := po.PaidAmount + req.Msg.Amount
		if err := tx.Model(po).Update("paid_amount", newPaid).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		po.PaidAmount = newPaid
		// CLOSED auto-transition if fully paid and fully received.
		if err := maybeCloseIfPaid(tx, po); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		paid = po.PaidAmount
		outstanding = po.OrderedTotal - po.PaidAmount
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}

	return connect.NewResponse(&purchasingifacev1.PayPurchaseResponse{
		PurchaseOrderId: req.Msg.PurchaseOrderId,
		PaidAmount:      paid,
		Outstanding:     outstanding,
	}), nil
}

func (p *PurchasePayments) GetSupplierBalances(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.GetSupplierBalancesRequest],
) (*connect.Response[purchasingifacev1.GetSupplierBalancesResponse], error) {
	type row struct {
		SupplierID   string `gorm:"column:supplier_id"`
		SupplierName string `gorm:"column:supplier_name"`
		Ordered      int64  `gorm:"column:ordered_total"`
		Paid         int64  `gorm:"column:paid_total"`
		Outstanding  int64
		OpenPOCount  int32 `gorm:"column:open_po_count"`
	}
	var rows []row
	q := p.db.WithContext(ctx).
		Table("suppliers s").
		Joins("LEFT JOIN purchase_orders po ON po.supplier_id = s.id AND po.status != ?", poStatusVoided).
		Select(`s.id AS supplier_id,
		        s.name AS supplier_name,
		        COALESCE(SUM(po.ordered_total), 0) AS ordered_total,
		        COALESCE(SUM(po.paid_amount), 0) AS paid_total,
		        COALESCE(SUM(po.ordered_total - po.paid_amount), 0) AS outstanding,
		        COUNT(po.id) FILTER (WHERE po.status NOT IN (?, ?)) AS open_po_count`,
			poStatusClosed, poStatusVoided).
		Group("s.id, s.name").
		Where("s.active = ?", true)
	if req.Msg.OnlyOutstanding {
		q = q.Having("COALESCE(SUM(po.ordered_total - po.paid_amount), 0) > 0")
	}
	q = q.Order("outstanding DESC, s.name ASC")

	if err := q.Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*purchasingifacev1.SupplierBalance, 0, len(rows))
	for _, r := range rows {
		out = append(out, &purchasingifacev1.SupplierBalance{
			SupplierId:   r.SupplierID,
			SupplierName: r.SupplierName,
			OrderedTotal: r.Ordered,
			PaidTotal:    r.Paid,
			Outstanding:  r.Outstanding,
			OpenPoCount:  r.OpenPOCount,
		})
	}
	return connect.NewResponse(&purchasingifacev1.GetSupplierBalancesResponse{Balances: out}), nil
}

// suppress unused import warning on model when only used transitively above
var _ = model.Supplier{}
