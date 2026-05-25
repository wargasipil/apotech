import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { stockClient } from "../lib/clients";
import {
  type GetStockLevelsRequest,
  MovementType,
  type RecordMovementRequest,
} from "../gen/inventory_iface/v1/stock_pb";
import { batchKeys } from "./batches";

import { ALL_LIMIT, DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type MovementsQueryOpts = {
  batchId?: string;
  medicineId?: string;
  type?: MovementType;
  query?: string;
  fromUnix?: number;
  toUnix?: number;
  page?: number;
  pageSize?: number;
};

export const stockKeys = {
  all: ["stock"] as const,
  movements: (opts: {
    batchId: string;
    medicineId: string;
    type: MovementType;
    query: string;
    fromUnix: number;
    toUnix: number;
    page: number;
    pageSize: number;
  }) => [...stockKeys.all, "movements", opts] as const,
  levels: (medicineId?: string) => [...stockKeys.all, "levels", medicineId ?? ""] as const,
};

// Server-paginated. Returns { rows, total }.
export function useMovementsQuery(opts: MovementsQueryOpts = {}) {
  const {
    batchId = "",
    medicineId = "",
    type = MovementType.UNSPECIFIED,
    query = "",
    fromUnix = 0,
    toUnix = 0,
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
  } = opts;
  const q = useQuery({
    queryKey: stockKeys.movements({ batchId, medicineId, type, query, fromUnix, toUnix, page, pageSize }),
    queryFn: async () => {
      const res = await stockClient.listMovements({
        batchId,
        medicineId,
        type,
        query,
        fromUnix: BigInt(fromUnix),
        toUnix: BigInt(toUnix),
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.movements, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Imperative one-shot fetch of ALL movements matching the filters (cap
// ALL_LIMIT), for CSV export. Not a hook — call from an export handler.
export async function fetchMovementsForExport(opts: MovementsQueryOpts = {}) {
  const {
    batchId = "",
    medicineId = "",
    type = MovementType.UNSPECIFIED,
    query = "",
    fromUnix = 0,
    toUnix = 0,
  } = opts;
  const res = await stockClient.listMovements({
    batchId,
    medicineId,
    type,
    query,
    fromUnix: BigInt(fromUnix),
    toUnix: BigInt(toUnix),
    limit: ALL_LIMIT,
    offset: 0,
  });
  return res.movements;
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
