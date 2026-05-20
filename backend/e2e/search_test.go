package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
)

// Search* RPCs power the async-mode `<SearchableSelect>` in the frontend.
// Each test creates a uniquely-named row, then asserts:
//   1. searching for a substring returns the row
//   2. searching for an unrelated nonsense string returns no matches
//   3. an empty query returns at least one row (popover-on-open behaviour)
//
// Tests share the dev DB and create rows tagged with a unique prefix per
// run, so concurrent runs / re-runs don't conflict.

func authReq[T any](env *Env, t *testing.T, msg *T) *connect.Request[T] {
	t.Helper()
	r := connect.NewRequest(msg)
	r.Header().Set("Authorization", env.AuthHeader(t))
	return r
}

func TestSearchSuppliers(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	unique := fmt.Sprintf("e2e-supplier-%d", time.Now().UnixNano())
	createRes, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{
			Name:         unique,
			ContactEmail: unique + "@example.com",
		}))
	require.NoError(t, err)
	require.NotEmpty(t, createRes.Msg.Supplier.Id)
	t.Cleanup(func() {
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: createRes.Msg.Supplier.Id}))
	})

	// Substring match.
	hit, err := env.Suppliers.SearchSuppliers(ctx, authReq(env, t,
		&inventoryifacev1.SearchSuppliersRequest{Query: unique[len(unique)-10:]}))
	require.NoError(t, err)
	require.True(t,
		anyNamedSupplier(hit.Msg.Suppliers, unique),
		"expected %q in search results, got %d rows", unique, len(hit.Msg.Suppliers))

	// Nonsense query.
	miss, err := env.Suppliers.SearchSuppliers(ctx, authReq(env, t,
		&inventoryifacev1.SearchSuppliersRequest{Query: "zzzzzzz-no-match-zzzzz"}))
	require.NoError(t, err)
	require.False(t, anyNamedSupplier(miss.Msg.Suppliers, unique),
		"unrelated query should not return our supplier")

	// Empty query → at least our supplier (and probably others).
	open, err := env.Suppliers.SearchSuppliers(ctx, authReq(env, t,
		&inventoryifacev1.SearchSuppliersRequest{Query: ""}))
	require.NoError(t, err)
	require.NotEmpty(t, open.Msg.Suppliers, "empty query should return at least one row")
}

func TestSearchMedicines(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	unique := fmt.Sprintf("e2e-med-%d", time.Now().UnixNano())
	createRes, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku:       unique,
			Name:      unique + " name",
			Unit:      "tab",
			UnitPrice: 1000,
		}))
	require.NoError(t, err)
	require.NotEmpty(t, createRes.Msg.Medicine.Id)
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: createRes.Msg.Medicine.Id}))
	})

	// Search by SKU substring.
	bySKU, err := env.Medicines.SearchMedicines(ctx, authReq(env, t,
		&inventoryifacev1.SearchMedicinesRequest{Query: unique}))
	require.NoError(t, err)
	require.True(t, anyNamedMedicine(bySKU.Msg.Medicines, unique),
		"SKU substring search should return our medicine")

	// Nonsense query.
	miss, err := env.Medicines.SearchMedicines(ctx, authReq(env, t,
		&inventoryifacev1.SearchMedicinesRequest{Query: "zzz-impossible-zzz"}))
	require.NoError(t, err)
	require.False(t, anyNamedMedicine(miss.Msg.Medicines, unique))
}

func TestSearchBatches(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	// Make a medicine for the batch to reference.
	medUnique := fmt.Sprintf("e2e-bmed-%d", time.Now().UnixNano())
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: medUnique, Name: medUnique + " name", Unit: "tab", UnitPrice: 100,
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: med.Msg.Medicine.Id}))
	})

	batchNum := fmt.Sprintf("BATCH-%d", time.Now().UnixNano())
	_, err = env.Batches.CreateBatch(ctx, authReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId:      med.Msg.Medicine.Id,
			BatchNumber:     batchNum,
			ExpiryDate:      "2099-12-31",
			CostPrice:       50,
			InitialQuantity: 10,
		}))
	require.NoError(t, err)

	// Search by batch number.
	hit, err := env.Batches.SearchBatches(ctx, authReq(env, t,
		&inventoryifacev1.SearchBatchesRequest{Query: batchNum}))
	require.NoError(t, err)
	require.True(t, anyBatchNumbered(hit.Msg.Batches, batchNum),
		"search should match the batch number we just created")

	// Search by joined medicine name.
	medHit, err := env.Batches.SearchBatches(ctx, authReq(env, t,
		&inventoryifacev1.SearchBatchesRequest{Query: medUnique}))
	require.NoError(t, err)
	require.True(t, anyBatchNumbered(medHit.Msg.Batches, batchNum),
		"search should match across the medicine name via JOIN")

	// Scope to a different medicine: our batch should NOT appear.
	scoped, err := env.Batches.SearchBatches(ctx, authReq(env, t,
		&inventoryifacev1.SearchBatchesRequest{Query: "", MedicineId: "00000000-0000-0000-0000-000000000000"}))
	require.NoError(t, err)
	require.False(t, anyBatchNumbered(scoped.Msg.Batches, batchNum),
		"scoping by an unrelated medicine_id should exclude our batch")
}

// ---------- helpers ----------

func anyNamedSupplier(rows []*inventoryifacev1.Supplier, needle string) bool {
	for _, r := range rows {
		if strings.Contains(r.Name, needle) {
			return true
		}
	}
	return false
}

func anyNamedMedicine(rows []*inventoryifacev1.Medicine, needle string) bool {
	for _, r := range rows {
		if strings.Contains(r.Name, needle) || strings.Contains(r.Sku, needle) {
			return true
		}
	}
	return false
}

func anyBatchNumbered(rows []*inventoryifacev1.Batch, needle string) bool {
	for _, r := range rows {
		if strings.Contains(r.BatchNumber, needle) {
			return true
		}
	}
	return false
}
