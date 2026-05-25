package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	customerifacev1 "github.com/apotech/backend/gen/customer_iface/v1"
	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
)

// TestResolveByIDs covers the four Resolve<Domain> RPCs that back the frontend
// resolve-by-IDs name lookups. Each must: return refs for known ids, omit an
// unknown id, and return an empty list for empty input.
func TestResolveByIDs(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("RES%d", uniq%100000))

	supName := fmt.Sprintf("ResolveSup-%d", uniq)
	sup, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{
			Code: fmt.Sprintf("RSUP%d", uniq%100000), Name: supName,
		}))
	require.NoError(t, err)
	supID := sup.Msg.Supplier.Id

	medName := fmt.Sprintf("ResolveMed-%d", uniq)
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("res-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id

	custName := fmt.Sprintf("ResolveCust-%d", uniq)
	cust, err := env.Customers.CreateCustomer(ctx, authReq(env, t,
		&customerifacev1.CreateCustomerRequest{Name: custName}))
	require.NoError(t, err)
	custID := cust.Msg.Customer.Id

	batchNo := fmt.Sprintf("RESBATCH-%d", uniq)
	batch, err := env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: batchNo, ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 10,
		}, wh))
	require.NoError(t, err)
	batchID := batch.Msg.Batch.Id

	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: supID}))
		_, _ = env.Customers.ArchiveCustomer(ctx, authReq(env, t,
			&customerifacev1.ArchiveCustomerRequest{Id: custID}))
	})

	// A valid-format UUID that does not exist — must be omitted from results.
	const unknown = "00000000-0000-0000-0000-000000000000"

	t.Run("medicines", func(t *testing.T) {
		res, err := env.Medicines.ResolveMedicines(ctx, authReq(env, t,
			&inventoryifacev1.ResolveMedicinesRequest{Ids: []string{medID, unknown}}))
		require.NoError(t, err)
		require.Len(t, res.Msg.Medicines, 1, "unknown id omitted")
		require.Equal(t, medID, res.Msg.Medicines[0].Id)
		require.Equal(t, medName, res.Msg.Medicines[0].Name)

		empty, err := env.Medicines.ResolveMedicines(ctx, authReq(env, t,
			&inventoryifacev1.ResolveMedicinesRequest{Ids: nil}))
		require.NoError(t, err)
		require.Empty(t, empty.Msg.Medicines, "empty ids -> empty list")
	})

	t.Run("suppliers", func(t *testing.T) {
		res, err := env.Suppliers.ResolveSuppliers(ctx, authReq(env, t,
			&inventoryifacev1.ResolveSuppliersRequest{Ids: []string{supID, unknown}}))
		require.NoError(t, err)
		require.Len(t, res.Msg.Suppliers, 1, "unknown id omitted")
		require.Equal(t, supID, res.Msg.Suppliers[0].Id)
		require.Equal(t, supName, res.Msg.Suppliers[0].Name)

		empty, err := env.Suppliers.ResolveSuppliers(ctx, authReq(env, t,
			&inventoryifacev1.ResolveSuppliersRequest{Ids: []string{}}))
		require.NoError(t, err)
		require.Empty(t, empty.Msg.Suppliers, "empty ids -> empty list")
	})

	t.Run("customers", func(t *testing.T) {
		res, err := env.Customers.ResolveCustomers(ctx, authReq(env, t,
			&customerifacev1.ResolveCustomersRequest{Ids: []string{custID, unknown}}))
		require.NoError(t, err)
		require.Len(t, res.Msg.Customers, 1, "unknown id omitted")
		require.Equal(t, custID, res.Msg.Customers[0].Id)
		require.Equal(t, custName, res.Msg.Customers[0].Name)

		empty, err := env.Customers.ResolveCustomers(ctx, authReq(env, t,
			&customerifacev1.ResolveCustomersRequest{}))
		require.NoError(t, err)
		require.Empty(t, empty.Msg.Customers, "empty ids -> empty list")
	})

	t.Run("batches", func(t *testing.T) {
		res, err := env.Batches.ResolveBatches(ctx, authReq(env, t,
			&inventoryifacev1.ResolveBatchesRequest{Ids: []string{batchID, unknown}}))
		require.NoError(t, err)
		require.Len(t, res.Msg.Batches, 1, "unknown id omitted")
		require.Equal(t, batchID, res.Msg.Batches[0].Id)
		require.Equal(t, batchNo, res.Msg.Batches[0].BatchNumber)
		require.Equal(t, medID, res.Msg.Batches[0].MedicineId)
		require.Equal(t, medName, res.Msg.Batches[0].MedicineName, "batch ref joins the medicine name")

		empty, err := env.Batches.ResolveBatches(ctx, authReq(env, t,
			&inventoryifacev1.ResolveBatchesRequest{Ids: nil}))
		require.NoError(t, err)
		require.Empty(t, empty.Msg.Batches, "empty ids -> empty list")
	})
}
