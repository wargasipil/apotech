import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { prescriptionClient } from "../lib/clients";
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

export function usePrescriptionsQuery(req: PartialMessage<ListPrescriptionsRequest> = {}) {
  return useQuery({
    queryKey: prescriptionKeys.list(req),
    queryFn: async () => {
      const res = await prescriptionClient.listPrescriptions(req);
      return res.prescriptions;
    },
  });
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
