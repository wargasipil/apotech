import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { supplierClient } from "../lib/clients";
import type {
  ArchiveSupplierRequest,
  CreateSupplierRequest,
  UpdateSupplierRequest,
} from "../gen/inventory_iface/v1/supplier_pb";

export const supplierKeys = {
  all: ["suppliers"] as const,
  list: (includeInactive: boolean) =>
    [...supplierKeys.all, "list", { includeInactive }] as const,
  search: (query: string) => [...supplierKeys.all, "search", query] as const,
};

export function useSuppliersQuery(includeInactive = false) {
  return useQuery({
    queryKey: supplierKeys.list(includeInactive),
    queryFn: async () => {
      const res = await supplierClient.listSuppliers({ includeInactive });
      return res.suppliers;
    },
  });
}

// Imperative search — call directly from <SearchableSelect loadOptions={...}>
// rather than via a hook (one call per debounced keystroke, no need to memoize
// in React Query). Returns the slice of matching suppliers (max 20).
export async function searchSuppliers(query: string) {
  const res = await supplierClient.searchSuppliers({ query, limit: 20 });
  return res.suppliers;
}

export function useCreateSupplierMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateSupplierRequest>) =>
      supplierClient.createSupplier(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: supplierKeys.all }),
  });
}

export function useUpdateSupplierMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateSupplierRequest>) =>
      supplierClient.updateSupplier(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: supplierKeys.all }),
  });
}

export function useArchiveSupplierMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchiveSupplierRequest>) =>
      supplierClient.archiveSupplier(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: supplierKeys.all }),
  });
}
