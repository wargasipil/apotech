import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { batchClient } from "../lib/clients";
import type {
  CreateBatchRequest,
  UpdateBatchRequest,
} from "../gen/inventory_iface/v1/batch_pb";

import { ALL_LIMIT, DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type BatchesQueryOpts = {
  medicineId?: string;
  onlyInStock?: boolean;
  query?: string;
  fromUnix?: number;
  toUnix?: number;
  dateField?: string; // "received" | "expiry"
  page?: number;
  pageSize?: number;
};

export const batchKeys = {
  all: ["batches"] as const,
  list: (opts: Required<BatchesQueryOpts>) =>
    [...batchKeys.all, "list", opts] as const,
};

// Server-paginated. Returns { rows, total }. For page-level maps pass
// { pageSize: ALL_LIMIT } or use useAllBatchesQuery.
export function useBatchesQuery(opts: BatchesQueryOpts = {}) {
  const {
    medicineId = "",
    onlyInStock = false,
    query = "",
    fromUnix = 0,
    toUnix = 0,
    dateField = "",
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
  } = opts;
  const q = useQuery({
    queryKey: batchKeys.list({ medicineId, onlyInStock, query, fromUnix, toUnix, dateField, page, pageSize }),
    queryFn: async () => {
      const res = await batchClient.listBatches({
        medicineId,
        onlyInStock,
        query,
        fromUnix: BigInt(fromUnix),
        toUnix: BigInt(toUnix),
        dateField,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.batches, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useAllBatchesQuery(opts: Omit<BatchesQueryOpts, "page" | "pageSize"> = {}) {
  return useBatchesQuery({ ...opts, pageSize: ALL_LIMIT });
}

// Imperative search — call directly from <SearchableSelect loadOptions={...}>.
// Optional medicineId scopes the search to a single medicine's batches.
export async function searchBatches(query: string, medicineId?: string) {
  const res = await batchClient.searchBatches({
    query,
    limit: 20,
    medicineId: medicineId ?? "",
  });
  return res.batches;
}

export function useCreateBatchMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateBatchRequest>) =>
      batchClient.createBatch(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: batchKeys.all }),
  });
}

export function useUpdateBatchMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateBatchRequest>) =>
      batchClient.updateBatch(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: batchKeys.all }),
  });
}
