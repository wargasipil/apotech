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
	stocktakeifacev1 "github.com/apotech/backend/gen/stocktake_iface/v1"
)

// TestListMedicines_OpnameBeforeFilter pins the `opname_before` filter on
// ListMedicines: medicines whose latest COMPLETED stocktake in the active
// warehouse is before the given date stay; medicines counted on/after the
// date drop out; never-counted medicines always stay (audit-overdue semantics).
func TestListMedicines_OpnameBeforeFilter(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	cleanupLeftoverDrafts(t, env, ctx)

	uniq := time.Now().UnixNano()
	medA, batchIDsA := seedMedicineAndBatches(t, env, ctx,
		fmt.Sprintf("e2e-flt-A-%d", uniq), []int32{10})
	medB, _ := seedMedicineAndBatches(t, env, ctx,
		fmt.Sprintf("e2e-flt-B-%d", uniq), []int32{5})

	// Stocktake medicine A today (variance 0 → still counts as a touched line).
	start, err := env.Stocktakes.StartStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.StartStocktakeRequest{Name: "filter test"}))
	require.NoError(t, err)
	sessID := start.Msg.Session.Id
	t.Cleanup(func() {
		_, _ = env.Stocktakes.VoidStocktake(ctx, authReq(env, t,
			&stocktakeifacev1.VoidStocktakeRequest{SessionId: sessID}))
	})
	_, err = env.Stocktakes.AddBatchesToSession(ctx, authReq(env, t,
		&stocktakeifacev1.AddBatchesToSessionRequest{SessionId: sessID, BatchIds: batchIDsA}))
	require.NoError(t, err)
	get, err := env.Stocktakes.GetStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.GetStocktakeRequest{Id: sessID}))
	require.NoError(t, err)
	require.Len(t, get.Msg.Lines, 1)
	_, err = env.Stocktakes.RecordCount(ctx, authReq(env, t,
		&stocktakeifacev1.RecordCountRequest{LineId: get.Msg.Lines[0].Id, CountedQty: 10}))
	require.NoError(t, err)
	_, err = env.Stocktakes.CompleteStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.CompleteStocktakeRequest{SessionId: sessID}))
	require.NoError(t, err)

	// helper: list with opname_before + a query to scope to our seeded medicines.
	listIDs := func(before, query string) []string {
		res, err := env.Medicines.ListMedicines(ctx, authReq(env, t,
			&inventoryifacev1.ListMedicinesRequest{
				OpnameBefore: before,
				Query:        query,
				Limit:        100,
			}))
		require.NoError(t, err)
		ids := make([]string, 0, len(res.Msg.Medicines))
		for _, m := range res.Msg.Medicines {
			ids = append(ids, m.Id)
		}
		return ids
	}

	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	// Both A (counted today, today < tomorrow) and B (never counted) appear.
	// Scope via the shared `e2e-flt-` prefix substring matches both.
	got := listIDs(tomorrow, fmt.Sprintf("e2e-flt-A-%d", uniq))
	require.Contains(t, got, medA, "A counted < tomorrow → kept")
	gotB := listIDs(tomorrow, fmt.Sprintf("e2e-flt-B-%d", uniq))
	require.Contains(t, gotB, medB, "B never counted → kept")

	// Filter "before yesterday": A was counted today (>= yesterday) → drops out; B never counted → stays.
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	gotA2 := listIDs(yesterday, fmt.Sprintf("e2e-flt-A-%d", uniq))
	require.NotContains(t, gotA2, medA, "A's last opname is today, not before yesterday → drops")
	gotB2 := listIDs(yesterday, fmt.Sprintf("e2e-flt-B-%d", uniq))
	require.Contains(t, gotB2, medB, "B never counted → stays under any opname_before")

	// Bogus date → InvalidArgument.
	_, err = env.Medicines.ListMedicines(ctx, authReq(env, t,
		&inventoryifacev1.ListMedicinesRequest{OpnameBefore: "bogus", Limit: 10}))
	require.Error(t, err)
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeInvalidArgument, cerr.Code())
}

// TestListMedicines_PopulatesLastStocktake covers the new enrichLastStocktake
// step: each page row carries the most recent COMPLETED stocktake date for
// the medicine in the active warehouse. Medicines never counted get "".
func TestListMedicines_PopulatesLastStocktake(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	cleanupLeftoverDrafts(t, env, ctx)

	uniq := time.Now().UnixNano()
	prefix := fmt.Sprintf("e2e-lst-%d", uniq)
	medA, batchIDsA := seedMedicineAndBatches(t, env, ctx, prefix+"-A", []int32{10})
	medB, _ := seedMedicineAndBatches(t, env, ctx, prefix+"-B", []int32{5})

	// Stocktake A today.
	start, err := env.Stocktakes.StartStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.StartStocktakeRequest{Name: "enrich test"}))
	require.NoError(t, err)
	sessID := start.Msg.Session.Id
	t.Cleanup(func() {
		_, _ = env.Stocktakes.VoidStocktake(ctx, authReq(env, t,
			&stocktakeifacev1.VoidStocktakeRequest{SessionId: sessID}))
	})
	_, err = env.Stocktakes.AddBatchesToSession(ctx, authReq(env, t,
		&stocktakeifacev1.AddBatchesToSessionRequest{SessionId: sessID, BatchIds: batchIDsA}))
	require.NoError(t, err)
	get, err := env.Stocktakes.GetStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.GetStocktakeRequest{Id: sessID}))
	require.NoError(t, err)
	_, err = env.Stocktakes.RecordCount(ctx, authReq(env, t,
		&stocktakeifacev1.RecordCountRequest{LineId: get.Msg.Lines[0].Id, CountedQty: 10}))
	require.NoError(t, err)
	_, err = env.Stocktakes.CompleteStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.CompleteStocktakeRequest{SessionId: sessID}))
	require.NoError(t, err)

	// List both medicines via a shared prefix query.
	res, err := env.Medicines.ListMedicines(ctx, authReq(env, t,
		&inventoryifacev1.ListMedicinesRequest{Query: prefix, Limit: 100}))
	require.NoError(t, err)
	byID := map[string]*inventoryifacev1.Medicine{}
	for _, m := range res.Msg.Medicines {
		byID[m.Id] = m
	}
	today := time.Now().Format("2006-01-02")
	require.Contains(t, byID, medA)
	require.Equal(t, today, byID[medA].LastStocktakeDate, "A counted today → list row carries today's date")
	require.Contains(t, byID, medB)
	require.Equal(t, "", byID[medB].LastStocktakeDate, "B never counted → list row carries empty")
}
