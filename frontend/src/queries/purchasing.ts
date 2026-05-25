import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import {
  purchaseOrderClient,
  purchasePaymentClient,
  purchaseReceiptClient,
} from "../lib/clients";
import type {
  CreatePurchaseOrderRequest,
  ListPurchaseOrdersRequest,
  UpdatePurchaseOrderRequest,
} from "../gen/purchasing_iface/v1/order_pb";
import type {
  CreateReceiptRequest,
  ListReceiptsRequest,
} from "../gen/purchasing_iface/v1/receipt_pb";
import type {
  GetSupplierBalancesRequest,
  PayPurchaseRequest,
} from "../gen/purchasing_iface/v1/payment_pb";
import { ALL_LIMIT } from "../lib/pagination";

export const purchasingKeys = {
  all: ["purchasing"] as const,
  orders: (filters: object) => [...purchasingKeys.all, "orders", filters] as const,
  order: (id: string) => [...purchasingKeys.all, "order", id] as const,
  receipts: (poId: string) => [...purchasingKeys.all, "receipts", poId] as const,
  balances: (filters: object) => [...purchasingKeys.all, "balances", filters] as const,
};

// ---------- Orders ----------
// Server-paginated. Returns { rows, total }. Caller sets limit/offset on req.
export function usePurchaseOrdersQuery(req: PartialMessage<ListPurchaseOrdersRequest> = {}) {
  const q = useQuery({
    queryKey: purchasingKeys.orders(req),
    queryFn: async () => {
      const res = await purchaseOrderClient.listPurchaseOrders(req);
      return { rows: res.orders, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Imperative one-shot fetch of ALL POs matching the filters (cap ALL_LIMIT), for
// CSV export. Not a hook — call from an export handler.
export async function fetchPurchaseOrdersForExport(
  req: PartialMessage<ListPurchaseOrdersRequest> = {},
) {
  const res = await purchaseOrderClient.listPurchaseOrders({ ...req, limit: ALL_LIMIT, offset: 0 });
  return res.orders;
}

export function usePurchaseOrderQuery(id: string, enabled = true) {
  return useQuery({
    queryKey: purchasingKeys.order(id),
    queryFn: async () => {
      const res = await purchaseOrderClient.getPurchaseOrder({ id });
      return res.order;
    },
    enabled: enabled && !!id,
  });
}

export function useCreatePurchaseOrderMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreatePurchaseOrderRequest>) =>
      purchaseOrderClient.createPurchaseOrder(req),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: purchasingKeys.all });
    },
  });
}

export function useUpdatePurchaseOrderMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdatePurchaseOrderRequest>) =>
      purchaseOrderClient.updatePurchaseOrder(req),
    onSuccess: (res) => {
      if (res.order?.id) {
        void qc.invalidateQueries({ queryKey: purchasingKeys.order(res.order.id) });
      }
      void qc.invalidateQueries({ queryKey: purchasingKeys.all });
    },
  });
}

export function useSendPurchaseOrderMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => purchaseOrderClient.sendPurchaseOrder({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: purchasingKeys.all }),
  });
}

export function useVoidPurchaseOrderMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => purchaseOrderClient.voidPurchaseOrder({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: purchasingKeys.all }),
  });
}

// ---------- Receipts ----------
export function useReceiptsQuery(req: PartialMessage<ListReceiptsRequest>) {
  return useQuery({
    queryKey: purchasingKeys.receipts(req.purchaseOrderId ?? ""),
    queryFn: async () => {
      const res = await purchaseReceiptClient.listReceipts(req);
      return res.receipts;
    },
    enabled: !!req.purchaseOrderId,
  });
}

export function useCreateReceiptMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateReceiptRequest>) =>
      purchaseReceiptClient.createReceipt(req),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: purchasingKeys.all });
      void qc.invalidateQueries({ queryKey: ["batches"] });
      void qc.invalidateQueries({ queryKey: ["stock"] });
    },
  });
}

// ---------- Payments ----------
export function usePayPurchaseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<PayPurchaseRequest>) =>
      purchasePaymentClient.payPurchase(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: purchasingKeys.all }),
  });
}

export function useSupplierBalancesQuery(req: PartialMessage<GetSupplierBalancesRequest> = {}) {
  return useQuery({
    queryKey: purchasingKeys.balances(req),
    queryFn: async () => {
      const res = await purchasePaymentClient.getSupplierBalances(req);
      return res.balances;
    },
  });
}
