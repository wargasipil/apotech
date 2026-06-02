import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { unitClient } from "../lib/clients";
import type {
  ArchiveUnitBaseRequest,
  ArchiveUnitDerivativeRequest,
  CreateUnitBaseRequest,
  CreateUnitDerivativeRequest,
  UpdateUnitBaseRequest,
  UpdateUnitDerivativeRequest,
} from "../gen/unit_iface/v1/unit_pb";

export const unitKeys = {
  all: ["units"] as const,
  bases: (includeInactive: boolean) =>
    [...unitKeys.all, "bases", includeInactive] as const,
};

// Global unit catalog. Returns base units (active by default) with their
// active derivatives hydrated. Used by Settings + the Medicines list popover.
export function useUnitBasesQuery(includeInactive = false) {
  return useQuery({
    queryKey: unitKeys.bases(includeInactive),
    queryFn: async () => {
      const res = await unitClient.listUnitBases({ includeInactive });
      return res.bases;
    },
    staleTime: 60_000,
  });
}

export function useCreateUnitBaseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateUnitBaseRequest>) =>
      unitClient.createUnitBase(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}

export function useUpdateUnitBaseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateUnitBaseRequest>) =>
      unitClient.updateUnitBase(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}

export function useArchiveUnitBaseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchiveUnitBaseRequest>) =>
      unitClient.archiveUnitBase(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}

export function useCreateUnitDerivativeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateUnitDerivativeRequest>) =>
      unitClient.createUnitDerivative(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}

export function useUpdateUnitDerivativeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateUnitDerivativeRequest>) =>
      unitClient.updateUnitDerivative(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}

export function useArchiveUnitDerivativeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchiveUnitDerivativeRequest>) =>
      unitClient.archiveUnitDerivative(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}
