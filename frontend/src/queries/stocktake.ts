import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { stocktakeClient } from "../lib/clients";
import type {
  AddAllInStockBatchesRequest,
  AddBatchesToSessionRequest,
  CompleteStocktakeRequest,
  RecordCountRequest,
  RemoveLineRequest,
  SetLineDispositionRequest,
  StartStocktakeRequest,
  VoidStocktakeRequest,
} from "../gen/stocktake_iface/v1/stocktake_pb";

export const stocktakeKeys = {
  all: ["stocktakes"] as const,
  list: (status: string) => [...stocktakeKeys.all, "list", { status }] as const,
  detail: (id: string) => [...stocktakeKeys.all, "detail", id] as const,
};

export function useStocktakesQuery(status = "") {
  return useQuery({
    queryKey: stocktakeKeys.list(status),
    queryFn: async () => {
      const res = await stocktakeClient.listStocktakes({ status, limit: 100 });
      return res.sessions;
    },
  });
}

export function useStocktakeQuery(id: string | undefined) {
  return useQuery({
    queryKey: stocktakeKeys.detail(id ?? ""),
    enabled: !!id,
    queryFn: async () => {
      const res = await stocktakeClient.getStocktake({ id: id ?? "" });
      return res;
    },
  });
}

export function useStartStocktakeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<StartStocktakeRequest>) =>
      stocktakeClient.startStocktake(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: stocktakeKeys.all }),
  });
}

export function useAddBatchesMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<AddBatchesToSessionRequest>) =>
      stocktakeClient.addBatchesToSession(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
      qc.invalidateQueries({ queryKey: stocktakeKeys.all });
    },
  });
}

export function useAddAllInStockBatchesMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<AddAllInStockBatchesRequest>) =>
      stocktakeClient.addAllInStockBatches(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
      qc.invalidateQueries({ queryKey: stocktakeKeys.all });
    },
  });
}

export function useRecordCountMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<RecordCountRequest>) =>
      stocktakeClient.recordCount(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
    },
  });
}

export function useSetLineDispositionMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetLineDispositionRequest>) =>
      stocktakeClient.setLineDisposition(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
    },
  });
}

export function useRemoveLineMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<RemoveLineRequest>) =>
      stocktakeClient.removeLine(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
    },
  });
}

export function useCompleteStocktakeMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CompleteStocktakeRequest>) =>
      stocktakeClient.completeStocktake(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
      qc.invalidateQueries({ queryKey: stocktakeKeys.all });
    },
  });
}

export function useVoidStocktakeMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<VoidStocktakeRequest>) =>
      stocktakeClient.voidStocktake(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
      qc.invalidateQueries({ queryKey: stocktakeKeys.all });
    },
  });
}
