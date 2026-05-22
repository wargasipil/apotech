import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { stockTransferClient } from "../lib/clients";
import type { CreateTransferRequest } from "../gen/warehouse_iface/v1/transfer_pb";

import { DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type TransfersQueryOpts = {
  warehouseId?: string;
  page?: number;
  pageSize?: number;
};

export const transferKeys = {
  all: ["transfers"] as const,
  list: (opts: Required<TransfersQueryOpts>) => [...transferKeys.all, "list", opts] as const,
  detail: (id: string) => [...transferKeys.all, "detail", id] as const,
};

// Server-paginated. Returns { rows, total }.
export function useTransfersQuery(opts: TransfersQueryOpts = {}) {
  const { warehouseId = "", page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: transferKeys.list({ warehouseId, page, pageSize }),
    queryFn: async () => {
      const res = await stockTransferClient.listTransfers({
        warehouseId,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.transfers, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useTransferQuery(id: string | undefined) {
  return useQuery({
    queryKey: transferKeys.detail(id ?? ""),
    enabled: !!id,
    queryFn: async () => {
      const res = await stockTransferClient.getTransfer({ id: id ?? "" });
      return res.transfer;
    },
  });
}

export function useCreateTransferMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateTransferRequest>) =>
      stockTransferClient.createTransfer(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: transferKeys.all });
      // Stock changed in two warehouses.
      qc.invalidateQueries({ queryKey: ["batches"] });
      qc.invalidateQueries({ queryKey: ["stock"] });
    },
  });
}
