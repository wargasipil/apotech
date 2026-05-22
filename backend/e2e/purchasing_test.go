package e2e

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	purchasingifacev1 "github.com/apotech/backend/gen/purchasing_iface/v1"
)

// TestSupplierCode covers the new unique supplier code: create requires it,
// search matches it, and a duplicate is rejected.
func TestSupplierCode(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	code := fmt.Sprintf("SUP-E2E-%d", time.Now().UnixNano()%1000000)

	created, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{Name: "Code test supplier", Code: code}))
	require.NoError(t, err)
	require.Equal(t, code, created.Msg.Supplier.Code)
	t.Cleanup(func() {
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: created.Msg.Supplier.Id}))
	})

	// Search by code substring returns it.
	hit, err := env.Suppliers.SearchSuppliers(ctx, authReq(env, t,
		&inventoryifacev1.SearchSuppliersRequest{Query: code}))
	require.NoError(t, err)
	found := false
	for _, s := range hit.Msg.Suppliers {
		if s.Id == created.Msg.Supplier.Id {
			found = true
		}
	}
	require.True(t, found, "search by code should return the supplier")

	// Missing code is rejected.
	_, err = env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{Name: "no code"}))
	require.Error(t, err)
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeInvalidArgument, cerr.Code())

	// Duplicate code is rejected.
	_, err = env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{Name: "dup", Code: code}))
	require.Error(t, err)
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeAlreadyExists, cerr.Code())
}

// TestPurchaseOrderListSearchAndReceipt covers the enriched PO list: search by
// PO no / supplier code / medicine name, the received-date filter, and that the
// list surfaces received_at + invoice_no + item medicine names.
func TestPurchaseOrderListSearchAndReceipt(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	supCode := fmt.Sprintf("PCODE%d", uniq%1000000)
	sup, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{Name: "PO test supplier", Code: supCode}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: sup.Msg.Supplier.Id}))
	})

	medName := fmt.Sprintf("PO-Med-%d", uniq)
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("po-sku-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	// Create + send a PO with one line.
	po, err := env.POs.CreatePurchaseOrder(ctx, authReq(env, t,
		&purchasingifacev1.CreatePurchaseOrderRequest{
			SupplierId: sup.Msg.Supplier.Id,
			Items: []*purchasingifacev1.PurchaseOrderItemInput{
				{MedicineId: medID, OrderedQty: 5, UnitCostPrice: 1000},
			},
		}))
	require.NoError(t, err)
	poID := po.Msg.Order.Id
	poNo := po.Msg.Order.PoNo
	require.Len(t, po.Msg.Order.Items, 1)
	poItemID := po.Msg.Order.Items[0].Id

	_, err = env.POs.SendPurchaseOrder(ctx, authReq(env, t,
		&purchasingifacev1.SendPurchaseOrderRequest{Id: poID}))
	require.NoError(t, err)

	// Receive the full qty with a supplier invoice number.
	invoice := fmt.Sprintf("FAK-%d", uniq%100000)
	_, err = env.Receipts.CreateReceipt(ctx, authReq(env, t,
		&purchasingifacev1.CreateReceiptRequest{
			PurchaseOrderId: poID,
			InvoiceNo:       invoice,
			Lines: []*purchasingifacev1.ReceiveLineInput{
				{PurchaseOrderItemId: poItemID, Qty: 5, ExpiryDate: "2099-12-31", BatchNumber: "PO-B1"},
			},
		}))
	require.NoError(t, err)

	// Search by PO number, supplier code, and medicine name each find the PO.
	for _, query := range []string{poNo, supCode, medName} {
		res, err := env.POs.ListPurchaseOrders(ctx, authReq(env, t,
			&purchasingifacev1.ListPurchaseOrdersRequest{Query: query}))
		require.NoError(t, err, "search %q", query)
		require.True(t, anyPO(res.Msg.Orders, poID), "search %q should return the PO", query)
	}

	// The enriched row carries received_at, invoice_no, and item medicine name.
	res, err := env.POs.ListPurchaseOrders(ctx, authReq(env, t,
		&purchasingifacev1.ListPurchaseOrdersRequest{Query: poNo}))
	require.NoError(t, err)
	got := findPO(res.Msg.Orders, poID)
	require.NotNil(t, got)
	require.Greater(t, got.ReceivedAt, int64(0), "received_at populated from the receipt")
	require.Equal(t, invoice, got.InvoiceNo)
	require.Len(t, got.Items, 1)
	require.Equal(t, medName, got.Items[0].MedicineName)

	// Received-date range filter includes it.
	from := time.Now().AddDate(0, 0, -1).Unix()
	to := time.Now().AddDate(0, 0, 2).Unix()
	res, err = env.POs.ListPurchaseOrders(ctx, authReq(env, t,
		&purchasingifacev1.ListPurchaseOrdersRequest{
			FromUnix: from, ToUnix: to, DateField: "received",
		}))
	require.NoError(t, err)
	require.True(t, anyPO(res.Msg.Orders, poID), "received-date range should include the PO")
}

func anyPO(rows []*purchasingifacev1.PurchaseOrder, id string) bool {
	return findPO(rows, id) != nil
}

func findPO(rows []*purchasingifacev1.PurchaseOrder, id string) *purchasingifacev1.PurchaseOrder {
	for _, r := range rows {
		if r.Id == id {
			return r
		}
	}
	return nil
}
