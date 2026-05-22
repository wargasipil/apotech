import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { taxInvoiceClient } from "../lib/clients";
import type {
  ImportNsfpRangeRequest,
  ListNsfpRequest,
  ListTaxInvoicesRequest,
} from "../gen/tax_iface/v1/tax_pb";

export const taxKeys = {
  all: ["tax"] as const,
  nsfp: (filters: object) => [...taxKeys.all, "nsfp", filters] as const,
  invoices: (filters: object) => [...taxKeys.all, "invoices", filters] as const,
};

// Server-paginated. Returns { rows, total, unusedTotal }. Caller sets
// limit/offset on req.
export function useNsfpQuery(req: PartialMessage<ListNsfpRequest> = {}) {
  const q = useQuery({
    queryKey: taxKeys.nsfp(req),
    queryFn: async () => {
      const res = await taxInvoiceClient.listNsfp(req);
      return { rows: res.entries, total: res.total, unusedTotal: res.unusedTotal };
    },
  });
  return {
    ...q,
    rows: q.data?.rows ?? [],
    total: q.data?.total ?? 0,
    unusedTotal: q.data?.unusedTotal ?? 0,
  };
}

export function useImportNsfpMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ImportNsfpRangeRequest>) =>
      taxInvoiceClient.importNsfpRange(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: taxKeys.all }),
  });
}

// Server-paginated. Returns { rows, total }. Caller sets limit/offset on req.
export function useTaxInvoicesQuery(req: PartialMessage<ListTaxInvoicesRequest> = {}) {
  const q = useQuery({
    queryKey: taxKeys.invoices(req),
    queryFn: async () => {
      const res = await taxInvoiceClient.listTaxInvoices(req);
      return { rows: res.invoices, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}
