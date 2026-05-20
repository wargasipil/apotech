import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { medicineClient } from "../lib/clients";
import type {
  ArchiveMedicineRequest,
  CreateMedicineRequest,
  UpdateMedicineRequest,
} from "../gen/inventory_iface/v1/medicine_pb";

export const medicineKeys = {
  all: ["medicines"] as const,
  list: (includeInactive: boolean) =>
    [...medicineKeys.all, "list", { includeInactive }] as const,
  prices: (medicineId: string) =>
    [...medicineKeys.all, "prices", medicineId] as const,
  search: (query: string) => [...medicineKeys.all, "search", query] as const,
};

export function useMedicinesQuery(includeInactive = false) {
  return useQuery({
    queryKey: medicineKeys.list(includeInactive),
    queryFn: async () => {
      const res = await medicineClient.listMedicines({ includeInactive });
      return res.medicines;
    },
  });
}

// Imperative search — call directly from <SearchableSelect loadOptions={...}>.
// Mirrors the SearchCustomers / searchSuppliers contract.
export async function searchMedicines(query: string) {
  const res = await medicineClient.searchMedicines({ query, limit: 20 });
  return res.medicines;
}

export function useMedicinePricesQuery(medicineId: string, enabled = true) {
  return useQuery({
    queryKey: medicineKeys.prices(medicineId),
    queryFn: async () => {
      const res = await medicineClient.listMedicinePrices({ medicineId });
      return res.prices;
    },
    enabled: enabled && !!medicineId,
  });
}

export function useCreateMedicineMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateMedicineRequest>) =>
      medicineClient.createMedicine(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: medicineKeys.all }),
  });
}

export function useUpdateMedicineMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateMedicineRequest>) =>
      medicineClient.updateMedicine(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: medicineKeys.all }),
  });
}

export function useArchiveMedicineMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchiveMedicineRequest>) =>
      medicineClient.archiveMedicine(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: medicineKeys.all }),
  });
}
