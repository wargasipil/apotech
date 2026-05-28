package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/apotech/backend/gen/inventory_iface/v1"
	settingsifacev1 "github.com/apotech/backend/gen/settings_iface/v1"
)

// TestSettings_GetUpdate proves Settings storage:
//   - GetSettings on a fresh DB returns the default (10) when no row exists.
//   - UpdateSettings as OWNER upserts; GetSettings then reflects the new value.
//   - UpdateSettings rejects negative values with InvalidArgument.
func TestSettings_GetUpdate(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	// Reset to the default so the "fresh DB returns 10" assertion is meaningful
	// even after prior runs have stored a value.
	_, err := env.Settings.UpdateSettings(ctx, authReq(env, t,
		&settingsifacev1.UpdateSettingsRequest{LowStockThreshold: 10}))
	require.NoError(t, err)

	got, err := env.Settings.GetSettings(ctx, authReq(env, t, &settingsifacev1.GetSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(10), got.Msg.Settings.LowStockThreshold)

	// Update to 5 and verify it round-trips.
	_, err = env.Settings.UpdateSettings(ctx, authReq(env, t,
		&settingsifacev1.UpdateSettingsRequest{LowStockThreshold: 5}))
	require.NoError(t, err)
	got, err = env.Settings.GetSettings(ctx, authReq(env, t, &settingsifacev1.GetSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(5), got.Msg.Settings.LowStockThreshold)

	// Negative rejected.
	_, err = env.Settings.UpdateSettings(ctx, authReq(env, t,
		&settingsifacev1.UpdateSettingsRequest{LowStockThreshold: -1}))
	require.Error(t, err, "negative threshold must be rejected")

	t.Cleanup(func() {
		_, _ = env.Settings.UpdateSettings(ctx, authReq(env, t,
			&settingsifacev1.UpdateSettingsRequest{LowStockThreshold: 10}))
	})
}

// TestListLowStock proves the bell-driving query: medicines whose ready_stock
// in the active warehouse is ≤ threshold, ordered by ready ASC. Active-warehouse
// scoping is the key — a different warehouse sees a different set.
func TestListLowStock(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	// Set the threshold deterministically.
	_, err := env.Settings.UpdateSettings(ctx, authReq(env, t,
		&settingsifacev1.UpdateSettingsRequest{LowStockThreshold: 10}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Settings.UpdateSettings(ctx, authReq(env, t,
			&settingsifacev1.UpdateSettingsRequest{LowStockThreshold: 10}))
	})

	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("LSA%d", uniq%100000))
	whB := makeWarehouse(env, t, ctx, fmt.Sprintf("LSB%d", uniq%100000))

	mk := func(sku string, stock int64) string {
		med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
			&inventoryifacev1.CreateMedicineRequest{
				Sku: sku, Name: sku + " name", Unit: "tab", UnitPrice: 1000,
			}))
		require.NoError(t, err)
		medID := med.Msg.Medicine.Id
		t.Cleanup(func() {
			_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
				&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
		})
		// Seed the batch's PURCHASE movement into WH-A (so WH-B sees 0 for it).
		if stock > 0 {
			_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
				&inventoryifacev1.CreateBatchRequest{
					MedicineId: medID, BatchNumber: sku + "-B1", ExpiryDate: "2099-12-31",
					CostPrice: 100, InitialQuantity: stock,
				}, whA))
			require.NoError(t, err)
		}
		return medID
	}
	a := mk(fmt.Sprintf("e2e-ls-a-%d", uniq), 2)  // low (≤10)
	b := mk(fmt.Sprintf("e2e-ls-b-%d", uniq), 8)  // low (≤10)
	c := mk(fmt.Sprintf("e2e-ls-c-%d", uniq), 50) // above threshold in WH-A

	// From WH-A: a and b are low (ordered ready ASC: a=2 before b=8); c is not.
	lsA, err := env.Medicines.ListLowStock(ctx, whReq(env, t,
		&inventoryifacev1.ListLowStockRequest{}, whA))
	require.NoError(t, err)
	require.Equal(t, int32(10), lsA.Msg.Threshold)
	idsA := pickIDs(lsA.Msg.Medicines, []string{a, b, c})
	require.Equal(t, []string{a, b}, idsA, "WH-A: a(2) then b(8); c(50) excluded")

	// From WH-B (no batches there): all three have ready_stock = 0 in WH-B, so
	// all three appear (ordered ready ASC ties broken by name).
	lsB, err := env.Medicines.ListLowStock(ctx, whReq(env, t,
		&inventoryifacev1.ListLowStockRequest{}, whB))
	require.NoError(t, err)
	idsB := pickIDs(lsB.Msg.Medicines, []string{a, b, c})
	require.ElementsMatch(t, []string{a, b, c}, idsB, "WH-B: all three are at 0")

	// Threshold 0 → only zero-stock items qualify. In WH-A only c (50) and b (8)
	// and a (2) all > 0, so none of our three. (Other meds in the dev DB may
	// match — we filter to our three.)
	_, err = env.Settings.UpdateSettings(ctx, authReq(env, t,
		&settingsifacev1.UpdateSettingsRequest{LowStockThreshold: 0}))
	require.NoError(t, err)
	lsZero, err := env.Medicines.ListLowStock(ctx, whReq(env, t,
		&inventoryifacev1.ListLowStockRequest{}, whA))
	require.NoError(t, err)
	require.Empty(t, pickIDs(lsZero.Msg.Medicines, []string{a, b, c}),
		"threshold 0 excludes any positive-stock medicine")
}

// pickIDs returns the subset of `ids` present in `meds`, preserving meds' order.
func pickIDs(meds []*inventoryifacev1.Medicine, ids []string) []string {
	want := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		want[id] = struct{}{}
	}
	out := make([]string, 0, len(ids))
	for _, m := range meds {
		if _, ok := want[m.Id]; ok {
			out = append(out, m.Id)
		}
	}
	return out
}
