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

export function useNsfpQuery(req: PartialMessage<ListNsfpRequest> = {}) {
  return useQuery({
    queryKey: taxKeys.nsfp(req),
    queryFn: async () => taxInvoiceClient.listNsfp(req),
  });
}

export function useImportNsfpMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ImportNsfpRangeRequest>) =>
      taxInvoiceClient.importNsfpRange(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: taxKeys.all }),
  });
}

export function useTaxInvoicesQuery(req: PartialMessage<ListTaxInvoicesRequest> = {}) {
  return useQuery({
    queryKey: taxKeys.invoices(req),
    queryFn: async () => {
      const res = await taxInvoiceClient.listTaxInvoices(req);
      return res.invoices;
    },
  });
}
