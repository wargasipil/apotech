import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { prescriptionClient } from "../lib/clients";
import { ALL_LIMIT } from "../lib/pagination";
import type {
  CreatePrescriptionRequest,
  ListPrescriptionsRequest,
  UpdatePrescriptionRequest,
} from "../gen/prescription_iface/v1/prescription_pb";

export const prescriptionKeys = {
  all: ["prescriptions"] as const,
  list: (filters: object) => [...prescriptionKeys.all, "list", filters] as const,
  one: (id: string) => [...prescriptionKeys.all, "one", id] as const,
};

// Server-paginated. Returns { rows, total }. Caller sets limit/offset on req.
export function usePrescriptionsQuery(req: PartialMessage<ListPrescriptionsRequest> = {}) {
  const q = useQuery({
    queryKey: prescriptionKeys.list(req),
    queryFn: async () => {
      const res = await prescriptionClient.listPrescriptions(req);
      return { rows: res.prescriptions, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function usePrescriptionQuery(id: string, enabled = true) {
  return useQuery({
    queryKey: prescriptionKeys.one(id),
    queryFn: async () => {
      const res = await prescriptionClient.getPrescription({ id });
      return res.prescription;
    },
    enabled: enabled && !!id,
  });
}

export function useCreatePrescriptionMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreatePrescriptionRequest>) =>
      prescriptionClient.createPrescription(req),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: prescriptionKeys.all });
    },
  });
}

export function useUpdatePrescriptionMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdatePrescriptionRequest>) =>
      prescriptionClient.updatePrescription(req),
    onSuccess: (res) => {
      if (res.prescription?.id) {
        void qc.invalidateQueries({ queryKey: prescriptionKeys.one(res.prescription.id) });
      }
      void qc.invalidateQueries({ queryKey: prescriptionKeys.all });
    },
  });
}

export function useVoidPrescriptionMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => prescriptionClient.voidPrescription({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: prescriptionKeys.all }),
  });
}

// useActiveRxCountQuery returns the count of ACTIVE prescriptions for the
// Dashboard "active Rx" tile. ListPrescriptions applies the status filter
// client-side over the fetched page (CLAUDE.md documents this), so we ask
// for ALL_LIMIT (1000) rows and read .rows.length. Beyond 1000 active Rx
// in a single pharmacy needs a dedicated count RPC; OK for now.
export function useActiveRxCountQuery() {
  const q = usePrescriptionsQuery({ status: "ACTIVE", limit: ALL_LIMIT });
  return { ...q, count: q.rows.length };
}
