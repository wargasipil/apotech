import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { supplierClient } from "../lib/clients";
import type {
  ArchiveSupplierRequest,
  CreateSupplierRequest,
  UpdateSupplierRequest,
} from "../gen/inventory_iface/v1/supplier_pb";

import { ALL_LIMIT, DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type SuppliersQueryOpts = {
  includeInactive?: boolean;
  query?: string;
  page?: number;
  pageSize?: number;
};

export const supplierKeys = {
  all: ["suppliers"] as const,
  list: (opts: Required<SuppliersQueryOpts>) =>
    [...supplierKeys.all, "list", opts] as const,
  search: (query: string) => [...supplierKeys.all, "search", query] as const,
};

// Server-paginated. Returns { rows, total }. For page-level name maps /
// preload selects pass { pageSize: ALL_LIMIT } or use useAllSuppliersQuery.
export function useSuppliersQuery(opts: SuppliersQueryOpts = {}) {
  const {
    includeInactive = false,
    query = "",
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
  } = opts;
  const q = useQuery({
    queryKey: supplierKeys.list({ includeInactive, query, page, pageSize }),
    queryFn: async () => {
      const res = await supplierClient.listSuppliers({
        includeInactive,
        query,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.suppliers, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useAllSuppliersQuery(includeInactive = false) {
  return useSuppliersQuery({ includeInactive, pageSize: ALL_LIMIT });
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
