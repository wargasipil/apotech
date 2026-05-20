import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { customerClient } from "../lib/clients";
import type {
  ArchiveCustomerRequest,
  CreateCustomerRequest,
  SearchCustomersRequest,
  UpdateCustomerRequest,
} from "../gen/customer_iface/v1/customer_pb";

export const customerKeys = {
  all: ["customers"] as const,
  list: (includeInactive: boolean) =>
    [...customerKeys.all, "list", { includeInactive }] as const,
  search: (query: string) => [...customerKeys.all, "search", query] as const,
};

export function useCustomersQuery(includeInactive = false) {
  return useQuery({
    queryKey: customerKeys.list(includeInactive),
    queryFn: async () => {
      const res = await customerClient.listCustomers({ includeInactive });
      return res.customers;
    },
  });
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
