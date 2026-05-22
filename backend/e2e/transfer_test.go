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
	warehouseifacev1 "github.com/apotech/backend/gen/warehouse_iface/v1"
)

// TestTransfer_MovesStockBetweenWarehouses proves a transfer reduces source
// stock and raises destination stock, with history + guards.
func TestTransfer_MovesStockBetweenWarehouses(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("TFA%d", uniq%100000))
	whB := makeWarehouse(env, t, ctx, fmt.Sprintf("TFB%d", uniq%100000))

	med, err := env.Medicines.CreateMedicine(ctx, authReq(env, t,
		&inventoryifacev1.CreateMedicineRequest{
			Sku: fmt.Sprintf("e2e-tf-%d", uniq), Name: "TF med", Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Medicine.Id
	t.Cleanup(func() {
		_, _ = env.Medicines.ArchiveMedicine(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveMedicineRequest{Id: medID}))
	})

	// Seed 10 units into warehouse A.
	batch, err := env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			MedicineId: medID, BatchNumber: "TF-B1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 10,
		}, whA))
	require.NoError(t, err)
	batchID := batch.Msg.Batch.Id

	// Transfer 4 from A to B.
	tr, err := env.Transfers.CreateTransfer(ctx, authReq(env, t,
		&warehouseifacev1.CreateTransferRequest{
			FromWarehouseId: whA, ToWarehouseId: whB, Note: "rebalance",
			Lines: []*warehouseifacev1.CreateTransferLineInput{{BatchId: batchID, Qty: 4}},
		}))
	require.NoError(t, err)
	require.NotEmpty(t, tr.Msg.Transfer.TransferNo, "transfer gets a TRF-… number")
	require.Len(t, tr.Msg.Transfer.Lines, 1)
	require.Equal(t, int32(4), tr.Msg.Transfer.Lines[0].Qty)

	// Stock moved.
	require.Equal(t, int64(6), stockOf(env, t, ctx, batchID, whA), "A drops by 4")
	require.Equal(t, int64(4), stockOf(env, t, ctx, batchID, whB), "B rises by 4")

	// History.
	list, err := env.Transfers.ListTransfers(ctx, authReq(env, t,
		&warehouseifacev1.ListTransfersRequest{WarehouseId: whB}))
	require.NoError(t, err)
	require.True(t, anyTransfer(list.Msg.Transfers, tr.Msg.Transfer.Id), "transfer shows in WH-B history")

	// Over-transfer is rejected; stock unchanged.
	_, err = env.Transfers.CreateTransfer(ctx, authReq(env, t,
		&warehouseifacev1.CreateTransferRequest{
			FromWarehouseId: whA, ToWarehouseId: whB,
			Lines: []*warehouseifacev1.CreateTransferLineInput{{BatchId: batchID, Qty: 999}},
		}))
	require.Error(t, err, "transferring more than available must fail")
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeFailedPrecondition, cerr.Code())
	require.Equal(t, int64(6), stockOf(env, t, ctx, batchID, whA), "failed transfer left A unchanged")

	// Same source + destination is rejected.
	_, err = env.Transfers.CreateTransfer(ctx, authReq(env, t,
		&warehouseifacev1.CreateTransferRequest{
			FromWarehouseId: whA, ToWarehouseId: whA,
			Lines: []*warehouseifacev1.CreateTransferLineInput{{BatchId: batchID, Qty: 1}},
		}))
	require.Error(t, err)
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeInvalidArgument, cerr.Code())
}

func anyTransfer(rows []*warehouseifacev1.StockTransfer, id string) bool {
	for _, r := range rows {
		if r.Id == id {
			return true
		}
	}
	return false
}
