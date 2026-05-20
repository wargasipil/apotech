import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { batchClient } from "../lib/clients";
import type {
  CreateBatchRequest,
  ListBatchesRequest,
  UpdateBatchRequest,
} from "../gen/inventory_iface/v1/batch_pb";

export const batchKeys = {
  all: ["batches"] as const,
  list: (filters: { medicineId?: string; onlyInStock?: boolean }) =>
    [...batchKeys.all, "list", filters] as const,
};

export function useBatchesQuery(filters: PartialMessage<ListBatchesRequest> = {}) {
  return useQuery({
    queryKey: batchKeys.list({
      medicineId: filters.medicineId ?? "",
      onlyInStock: filters.onlyInStock ?? false,
    }),
    queryFn: async () => {
      const res = await batchClient.listBatches(filters);
      return res.batches;
    },
  });
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
