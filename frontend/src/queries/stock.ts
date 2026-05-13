import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { stockClient } from "../lib/clients";
import {
  type GetStockLevelsRequest,
  type ListMovementsRequest,
  MovementType,
  type RecordMovementRequest,
} from "../gen/inventory_iface/v1/stock_pb";
import { batchKeys } from "./batches";

export const stockKeys = {
  all: ["stock"] as const,
  movements: (filters: { batchId?: string; type?: MovementType }) =>
    [...stockKeys.all, "movements", filters] as const,
  levels: (medicineId?: string) => [...stockKeys.all, "levels", medicineId ?? ""] as const,
};

export function useMovementsQuery(filters: PartialMessage<ListMovementsRequest> = {}) {
  return useQuery({
    queryKey: stockKeys.movements({
      batchId: filters.batchId ?? "",
      type: (filters.type as MovementType) ?? MovementType.UNSPECIFIED,
    }),
    queryFn: async () => {
      const res = await stockClient.listMovements(filters);
      return res.movements;
    },
  });
}

export function useStockLevelsQuery(req: PartialMessage<GetStockLevelsRequest> = {}) {
  return useQuery({
    queryKey: stockKeys.levels(req.medicineId),
    queryFn: async () => {
      const res = await stockClient.getStockLevels(req);
      return res.levels;
    },
  });
}

export function useRecordMovementMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<RecordMovementRequest>) =>
      stockClient.recordMovement(req),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: stockKeys.all });
      void qc.invalidateQueries({ queryKey: batchKeys.all });
    },
  });
}
