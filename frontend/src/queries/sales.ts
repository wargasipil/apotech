import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { saleClient } from "../lib/clients";
import type {
  AddItemRequest,
  AttachPrescriptionRequest,
  CompleteSaleRequest,
  DetachPrescriptionRequest,
  GetSalesSummaryRequest,
  ListSalesRequest,
  RemoveItemRequest,
  SetItemQuantityRequest,
  SetSaleCustomerRequest,
  VoidSaleRequest,
} from "../gen/pos_iface/v1/sale_pb";

export const saleKeys = {
  all: ["sales"] as const,
  list: (filters: object) => [...saleKeys.all, "list", filters] as const,
  summary: (filters: object) => [...saleKeys.all, "summary", filters] as const,
  detail: (id: string) => [...saleKeys.all, "detail", id] as const,
  todaySnapshot: () => [...saleKeys.all, "today-snapshot"] as const,
};

export function useStartSaleMutation() {
  return useMutation({
    mutationFn: () => saleClient.startSale({}),
  });
}

export function useAddItemMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<AddItemRequest>) => saleClient.addItem(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useSetItemQuantityMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetItemQuantityRequest>) =>
      saleClient.setItemQuantity(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useRemoveItemMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<RemoveItemRequest>) => saleClient.removeItem(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useSetSaleCustomerMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetSaleCustomerRequest>) =>
      saleClient.setSaleCustomer(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useAttachPrescriptionMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<AttachPrescriptionRequest>) =>
      saleClient.attachPrescription(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useDetachPrescriptionMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<DetachPrescriptionRequest>) =>
      saleClient.detachPrescription(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useCompleteSaleMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CompleteSaleRequest>) => saleClient.completeSale(req),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: saleKeys.all });
      void qc.invalidateQueries({ queryKey: ["stock"] });
      void qc.invalidateQueries({ queryKey: ["batches"] });
    },
  });
}

export function useVoidSaleMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<VoidSaleRequest>) => saleClient.voidSale(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: saleKeys.all }),
  });
}

export function usePrintReceiptMutation() {
  return useMutation({
    mutationFn: (saleId: string) => saleClient.printReceipt({ saleId }),
  });
}

// Server-paginated. Returns { rows, total }. Caller sets limit/offset on filters.
export function useListSalesQuery(filters: PartialMessage<ListSalesRequest> = {}) {
  const q = useQuery({
    queryKey: saleKeys.list(filters),
    queryFn: async () => {
      const res = await saleClient.listSales(filters);
      return { rows: res.sales, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Server-side aggregate over ALL rows matching the same filters as
// useListSalesQuery (NOT a client-side sum of a page). Backs the order-history
// summary bar.
export function useSalesSummaryQuery(filters: PartialMessage<GetSalesSummaryRequest> = {}) {
  return useQuery({
    queryKey: saleKeys.summary(filters),
    queryFn: () => saleClient.getSalesSummary(filters),
    staleTime: 30_000,
  });
}

export function useTodaySnapshotQuery() {
  return useQuery({
    queryKey: saleKeys.todaySnapshot(),
    queryFn: async () => saleClient.getTodaySnapshot({}),
    staleTime: 30_000,
  });
}
