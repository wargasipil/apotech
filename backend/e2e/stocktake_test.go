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
	"github.com/apotech/backend/internal/model"
)

// TestStocktake_CompleteFlow drives the full happy path:
//   start session → add 4 batches (stock 10/5/0/8) → record counts on three of
//   them (12/5/6) → flip one of them to WRITE_OFF with kind EXPIRED → complete
//   → assert exactly TWO stock_movements landed (qty +2 ADJUSTMENT, qty -2
//   WRITE_OFF), each linked to the right line, and that the un-counted and
//   zero-variance lines produced none.
func TestStocktake_CompleteFlow(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	cleanupLeftoverDrafts(t, env, ctx)

	med, batchIDs := seedMedicineAndBatches(t, env, ctx,
		fmt.Sprintf("e2e-st-%d", time.Now().UnixNano()),
		[]int32{10, 5, 0, 8},
	)
	_ = med

	// 1. Start session.
	start, err := env.Stocktakes.StartStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.StartStocktakeRequest{Name: "Test stocktake"}))
	require.NoError(t, err)
	sessID := start.Msg.Session.Id
	require.NotEmpty(t, sessID)
	require.Equal(t, "DRAFT", start.Msg.Session.Status)
	require.Equal(t, int32(0), start.Msg.Session.LineCount)
	t.Cleanup(func() {
		// Void if anything left it in DRAFT (e.g. an early failure before
		// Complete). Idempotent: a COMPLETED/VOIDED session rejects void.
		_, _ = env.Stocktakes.VoidStocktake(ctx, authReq(env, t,
			&stocktakeifacev1.VoidStocktakeRequest{SessionId: sessID}))
	})

	// 2. Add all 4 batches as lines.
	addRes, err := env.Stocktakes.AddBatchesToSession(ctx, authReq(env, t,
		&stocktakeifacev1.AddBatchesToSessionRequest{
			SessionId: sessID,
			BatchIds:  batchIDs,
		}))
	require.NoError(t, err)
	require.Equal(t, int32(4), addRes.Msg.AddedCount)
	require.Equal(t, int32(0), addRes.Msg.SkippedCount)

	// Re-add the same batches: all should be skipped.
	reAdd, err := env.Stocktakes.AddBatchesToSession(ctx, authReq(env, t,
		&stocktakeifacev1.AddBatchesToSessionRequest{
			SessionId: sessID,
			BatchIds:  batchIDs,
		}))
	require.NoError(t, err)
	require.Equal(t, int32(0), reAdd.Msg.AddedCount)
	require.Equal(t, int32(4), reAdd.Msg.SkippedCount)

	// 3. Get session and grab the 4 line IDs ordered by created_at (the order
	// AddBatches inserted them — same order as batchIDs).
	get, err := env.Stocktakes.GetStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.GetStocktakeRequest{Id: sessID}))
	require.NoError(t, err)
	require.Len(t, get.Msg.Lines, 4)

	// Lines come back ordered by created_at ASC, mapping 1:1 with batchIDs.
	lines := get.Msg.Lines
	expectedSnapshots := []int32{10, 5, 0, 8}
	for i, l := range lines {
		require.Equal(t, batchIDs[i], l.BatchId, "line %d should be for batch %d", i, i)
		require.Equal(t, expectedSnapshots[i], l.ExpectedQty,
			"line %d expected_qty should be snapshotted at add time", i)
		require.False(t, l.Counted, "line %d should start uncounted", i)
		require.Equal(t, "ADJUSTMENT", l.Disposition, "line %d should default to ADJUSTMENT", i)
	}

	// 4. RecordCount on lines 0, 1, 3 (skip line 2).
	counts := map[int]int32{0: 12, 1: 5, 3: 6}
	for idx, qty := range counts {
		rc, rerr := env.Stocktakes.RecordCount(ctx, authReq(env, t,
			&stocktakeifacev1.RecordCountRequest{LineId: lines[idx].Id, CountedQty: qty}))
		require.NoError(t, rerr, "RecordCount on line %d", idx)
		require.True(t, rc.Msg.Line.Counted)
		require.Equal(t, qty, rc.Msg.Line.CountedQty)
		require.Equal(t, qty-expectedSnapshots[idx], rc.Msg.Line.Variance)
	}

	// 5. Flip line 3 to WRITE_OFF with kind EXPIRED + note.
	setDisp, err := env.Stocktakes.SetLineDisposition(ctx, authReq(env, t,
		&stocktakeifacev1.SetLineDispositionRequest{
			LineId:          lines[3].Id,
			Disposition:     "WRITE_OFF",
			WriteOffKind:    "EXPIRED",
			DispositionNote: "expired batch from 2025",
		}))
	require.NoError(t, err)
	require.Equal(t, "WRITE_OFF", setDisp.Msg.Line.Disposition)
	require.Equal(t, "EXPIRED", setDisp.Msg.Line.WriteOffKind)
	require.Equal(t, "expired batch from 2025", setDisp.Msg.Line.DispositionNote)

	// 6. Complete.
	done, err := env.Stocktakes.CompleteStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.CompleteStocktakeRequest{SessionId: sessID}))
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", done.Msg.Session.Status)
	require.Equal(t, int32(2), done.Msg.MovementsWritten,
		"only line 0 (+2) and line 3 (-2) had non-zero variance")

	// 7. Assert stock_movements landed correctly.
	// Line 0: qty=+2, type=ADJUSTMENT, write_off_kind=NULL.
	var line0Move model.StockMovement
	err = env.DB.Where("stocktake_line_id = ?", lines[0].Id).First(&line0Move).Error
	require.NoError(t, err, "line 0 movement should exist")
	require.Equal(t, int32(2), line0Move.Qty)
	require.Equal(t, "ADJUSTMENT", line0Move.Type)
	require.Nil(t, line0Move.WriteOffKind, "ADJUSTMENT movement has no write_off_kind")
	require.Contains(t, line0Move.Reason, "Stocktake: Test stocktake")

	// Line 3: qty=-2, type=WRITE_OFF, write_off_kind=EXPIRED, reason has kind + note.
	var line3Move model.StockMovement
	err = env.DB.Where("stocktake_line_id = ?", lines[3].Id).First(&line3Move).Error
	require.NoError(t, err, "line 3 movement should exist")
	require.Equal(t, int32(-2), line3Move.Qty)
	require.Equal(t, "WRITE_OFF", line3Move.Type)
	require.NotNil(t, line3Move.WriteOffKind)
	require.Equal(t, "EXPIRED", *line3Move.WriteOffKind)
	require.Contains(t, line3Move.Reason, "EXPIRED")
	require.Contains(t, line3Move.Reason, "expired batch from 2025")

	// Line 1 (variance 0) and line 2 (uncounted) must NOT have produced movements.
	for _, l := range []*stocktakeifacev1.StocktakeLine{lines[1], lines[2]} {
		var c int64
		err := env.DB.Model(&model.StockMovement{}).
			Where("stocktake_line_id = ?", l.Id).Count(&c).Error
		require.NoError(t, err)
		require.Equal(t, int64(0), c, "line %s should not have produced a movement", l.Id)
	}
}

// TestStocktake_RejectsSecondDraft asserts the single-DRAFT-at-a-time rule.
func TestStocktake_RejectsSecondDraft(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	cleanupLeftoverDrafts(t, env, ctx)

	first, err := env.Stocktakes.StartStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.StartStocktakeRequest{Name: "first"}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Stocktakes.VoidStocktake(ctx, authReq(env, t,
			&stocktakeifacev1.VoidStocktakeRequest{SessionId: first.Msg.Session.Id}))
	})

	_, err = env.Stocktakes.StartStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.StartStocktakeRequest{Name: "second"}))
	require.Error(t, err, "starting a second DRAFT must fail")
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeFailedPrecondition, cerr.Code())
}

// TestStocktake_ValidationGuards asserts the two validation rules in
// CompleteStocktake + SetLineDisposition: positive variance can't be
// WRITE_OFF, and SetLineDisposition rejects WRITE_OFF without a kind.
func TestStocktake_ValidationGuards(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	cleanupLeftoverDrafts(t, env, ctx)

	_, batchIDs := seedMedicineAndBatches(t, env, ctx,
		fmt.Sprintf("e2e-stv-%d", time.Now().UnixNano()),
		[]int32{10},
	)

	start, err := env.Stocktakes.StartStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.StartStocktakeRequest{Name: "guard test"}))
	require.NoError(t, err)
	sessID := start.Msg.Session.Id
	t.Cleanup(func() {
		_, _ = env.Stocktakes.VoidStocktake(ctx, authReq(env, t,
			&stocktakeifacev1.VoidStocktakeRequest{SessionId: sessID}))
	})
	_, err = env.Stocktakes.AddBatchesToSession(ctx, authReq(env, t,
		&stocktakeifacev1.AddBatchesToSessionRequest{SessionId: sessID, BatchIds: batchIDs}))
	require.NoError(t, err)
	get, err := env.Stocktakes.GetStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.GetStocktakeRequest{Id: sessID}))
	require.NoError(t, err)
	require.Len(t, get.Msg.Lines, 1)
	line := get.Msg.Lines[0]

	// (a) SetLineDisposition rejects WRITE_OFF without a kind.
	_, err = env.Stocktakes.SetLineDisposition(ctx, authReq(env, t,
		&stocktakeifacev1.SetLineDispositionRequest{
			LineId:      line.Id,
			Disposition: "WRITE_OFF",
		}))
	require.Error(t, err, "WRITE_OFF without write_off_kind must be rejected")
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeInvalidArgument, cerr.Code())

	// (b) Count above expected (positive variance) then flag WRITE_OFF —
	// CompleteStocktake must reject. We achieve a positive variance + WRITE_OFF
	// via two RPCs: RecordCount(12) then SetLineDisposition(WRITE_OFF, EXPIRED).
	_, err = env.Stocktakes.RecordCount(ctx, authReq(env, t,
		&stocktakeifacev1.RecordCountRequest{LineId: line.Id, CountedQty: 12}))
	require.NoError(t, err)
	_, err = env.Stocktakes.SetLineDisposition(ctx, authReq(env, t,
		&stocktakeifacev1.SetLineDispositionRequest{
			LineId:       line.Id,
			Disposition:  "WRITE_OFF",
			WriteOffKind: "OTHER",
		}))
	require.NoError(t, err)

	_, err = env.Stocktakes.CompleteStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.CompleteStocktakeRequest{SessionId: sessID}))
	require.Error(t, err, "positive variance + WRITE_OFF must fail Complete")
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeFailedPrecondition, cerr.Code())
}

// TestStocktake_Void marks a DRAFT session VOIDED and writes no movements.
func TestStocktake_Void(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	cleanupLeftoverDrafts(t, env, ctx)

	_, batchIDs := seedMedicineAndBatches(t, env, ctx,
		fmt.Sprintf("e2e-stvd-%d", time.Now().UnixNano()),
		[]int32{10},
	)

	start, err := env.Stocktakes.StartStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.StartStocktakeRequest{Name: "to void"}))
	require.NoError(t, err)
	sessID := start.Msg.Session.Id

	_, err = env.Stocktakes.AddBatchesToSession(ctx, authReq(env, t,
		&stocktakeifacev1.AddBatchesToSessionRequest{SessionId: sessID, BatchIds: batchIDs}))
	require.NoError(t, err)
	get, err := env.Stocktakes.GetStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.GetStocktakeRequest{Id: sessID}))
	require.NoError(t, err)
	_, err = env.Stocktakes.RecordCount(ctx, authReq(env, t,
		&stocktakeifacev1.RecordCountRequest{LineId: get.Msg.Lines[0].Id, CountedQty: 8}))
	require.NoError(t, err)

	voidRes, err := env.Stocktakes.VoidStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.VoidStocktakeRequest{SessionId: sessID}))
	require.NoError(t, err)
	require.Equal(t, "VOIDED", voidRes.Msg.Session.Status)

	// No stock_movements should have been written for this session.
	var c int64
	err = env.DB.Model(&model.StockMovement{}).
		Where("stocktake_line_id = ?", get.Msg.Lines[0].Id).Count(&c).Error
	require.NoError(t, err)
	require.Equal(t, int64(0), c, "Voided session must not produce movements")
}

// ---------- test helpers ----------

// cleanupLeftoverDrafts voids any DRAFT stocktake session the global rule
// would otherwise block on. The single-DRAFT-at-a-time rule is global; a
// failed prior test run can leave one behind.
func cleanupLeftoverDrafts(t *testing.T, env *Env, ctx context.Context) {
	t.Helper()
	list, err := env.Stocktakes.ListStocktakes(ctx, authReq(env, t,
		&stocktakeifacev1.ListStocktakesRequest{Status: "DRAFT", Limit: 50}))
	if err != nil {
		t.Fatalf("list leftover drafts: %v", err)
	}
	for _, s := range list.Msg.Sessions {
		_, _ = env.Stocktakes.VoidStocktake(ctx, authReq(env, t,
			&stocktakeifacev1.VoidStocktakeRequest{SessionId: s.Id}))
	}
}

// seedMedicineAndBatches creates one medicine and N batches with the given
// initial-stock quantities (0 is allowed). Returns the medicine id and the
// batch ids in the same order as quantities.
func seedMedicineAndBatches(
	t *testing.T,
	env *Env,
	ctx context.Context,
	prefix string,
	quantities []int32,
) (string, []string) {
	t.Helper()
	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: prefix, Name: prefix + " name", Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	batchIDs := make([]string, 0, len(quantities))
	for i, q := range quantities {
		bn := fmt.Sprintf("%s-B%d-%d", prefix, i, time.Now().UnixNano())
		b, err := env.Batches.CreateBatch(ctx, authReq(env, t,
			&inventoryifacev1.CreateBatchRequest{
				MedicineId:      medID,
				BatchNumber:     bn,
				ExpiryDate:      "2099-12-31",
				CostPrice:       100,
				InitialQuantity: int64(q),
			}))
		require.NoError(t, err)
		batchIDs = append(batchIDs, b.Msg.Batch.Id)
	}
	return medID, batchIDs
}
