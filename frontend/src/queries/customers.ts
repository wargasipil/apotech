import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { customerClient } from "../lib/clients";
import type {
  ArchiveCustomerRequest,
  CreateCustomerRequest,
  SearchCustomersRequest,
  UpdateCustomerRequest,
} from "../gen/customer_iface/v1/customer_pb";

import { ALL_LIMIT, DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type CustomersQueryOpts = {
  includeInactive?: boolean;
  query?: string;
  page?: number;
  pageSize?: number;
};

export const customerKeys = {
  all: ["customers"] as const,
  list: (opts: Required<CustomersQueryOpts>) =>
    [...customerKeys.all, "list", opts] as const,
  search: (query: string) => [...customerKeys.all, "search", query] as const,
};

// Server-paginated. Returns { rows, total }. For page-level name maps pass
// { pageSize: ALL_LIMIT } or use useAllCustomersQuery.
export function useCustomersQuery(opts: CustomersQueryOpts = {}) {
  const {
    includeInactive = false,
    query = "",
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
  } = opts;
  const q = useQuery({
    queryKey: customerKeys.list({ includeInactive, query, page, pageSize }),
    queryFn: async () => {
      const res = await customerClient.listCustomers({
        includeInactive,
        query,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.customers, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useAllCustomersQuery(includeInactive = false) {
  return useCustomersQuery({ includeInactive, pageSize: ALL_LIMIT });
}

export function useCustomerSearchQuery(query: string, enabled = true) {
  return useQuery({
    queryKey: customerKeys.search(query),
    queryFn: async () => {
      const res = await customerClient.searchCustomers({ query, limit: 20 });
      return res.customers;
    },
    enabled,
    staleTime: 10_000,
  });
}

// Imperative search — call directly from <SearchableSelect loadOptions={...}>.
// Mirrors the searchSuppliers / searchMedicines / searchBatches contract.
export async function searchCustomers(query: string) {
  const res = await customerClient.searchCustomers({ query, limit: 20 });
  return res.customers;
}

export function useCreateCustomerMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateCustomerRequest>) =>
      customerClient.createCustomer(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: customerKeys.all }),
  });
}

export function useUpdateCustomerMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateCustomerRequest>) =>
      customerClient.updateCustomer(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: customerKeys.all }),
  });
}

export function useArchiveCustomerMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchiveCustomerRequest>) =>
      customerClient.archiveCustomer(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: customerKeys.all }),
  });
}

// Keep SearchCustomersRequest re-exported for callers that need the type.
export type { SearchCustomersRequest };
