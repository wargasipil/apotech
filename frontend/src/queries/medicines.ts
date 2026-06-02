import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { medicineClient } from "../lib/clients";
import type {
  ArchiveMedicineRequest,
  CreateMedicineRequest,
  UpdateMedicineRequest,
} from "../gen/inventory_iface/v1/medicine_pb";

import { ALL_LIMIT, DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type MedicinesQueryOpts = {
  includeInactive?: boolean;
  query?: string;
  opnameBefore?: string; // YYYY-MM-DD; filter to medicines counted before this date OR never counted
  page?: number;
  pageSize?: number;
};

export const medicineKeys = {
  all: ["medicines"] as const,
  list: (opts: Required<MedicinesQueryOpts>) =>
    [...medicineKeys.all, "list", opts] as const,
  one: (id: string) => [...medicineKeys.all, "one", id] as const,
  prices: (medicineId: string) =>
    [...medicineKeys.all, "prices", medicineId] as const,
  unitPrices: (medicineId: string) =>
    [...medicineKeys.all, "unitPrices", medicineId] as const,
  search: (query: string) => [...medicineKeys.all, "search", query] as const,
};

// Low-stock list for the TopBar bell — medicines whose ready_stock in the
// caller's active warehouse is <= the configured threshold. Polls every 60s;
// also auto-refetches on warehouse switch (existing invalidateQueries) and on
// threshold update (the settings mutation invalidates ["lowStock"]).
export function useLowStockQuery(opts: { enabled?: boolean } = {}) {
  const q = useQuery({
    queryKey: ["lowStock"],
    queryFn: async () => {
      const res = await medicineClient.listLowStock({});
      return { medicines: res.medicines, threshold: res.threshold, total: res.total };
    },
    enabled: opts.enabled ?? true,
    refetchInterval: 60_000,
    staleTime: 30_000,
    meta: { silentError: true },
  });
  return q;
}

// Single medicine (detail page). GetMedicine is stock-enriched server-side
// (ready_stock for the active warehouse + on_order_stock).
export function useMedicineQuery(id: string, enabled = true) {
  return useQuery({
    queryKey: medicineKeys.one(id),
    queryFn: async () => {
      const res = await medicineClient.getMedicine({ id });
      return res.medicine;
    },
    enabled: enabled && !!id,
  });
}

// Server-paginated. Returns { rows, total } plus the React Query state.
// For page-level name maps / preload selects pass { pageSize: ALL_LIMIT }.
export function useMedicinesQuery(opts: MedicinesQueryOpts = {}) {
  const {
    includeInactive = false,
    query = "",
    opnameBefore = "",
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
  } = opts;
  const q = useQuery({
    queryKey: medicineKeys.list({ includeInactive, query, opnameBefore, page, pageSize }),
    queryFn: async () => {
      const res = await medicineClient.listMedicines({
        includeInactive,
        query,
        opnameBefore,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.medicines, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Convenience for page-level name maps / preload selects that need the full list.
export function useAllMedicinesQuery(includeInactive = false) {
  return useMedicinesQuery({ includeInactive, pageSize: ALL_LIMIT });
}

// Imperative search — call directly from <SearchableSelect loadOptions={...}>.
// Mirrors the SearchCustomers / searchSuppliers contract.
export async function searchMedicines(query: string) {
  const res = await medicineClient.searchMedicines({ query, limit: 20 });
  return res.medicines;
}

// Imperative one-shot fetch of ALL medicines matching the filter (cap
// ALL_LIMIT), for CSV export. Not a hook — call from an export handler.
export async function fetchMedicinesForExport(opts: MedicinesQueryOpts = {}) {
  const { includeInactive = false, query = "", opnameBefore = "" } = opts;
  const res = await medicineClient.listMedicines({
    includeInactive,
    query,
    opnameBefore,
    limit: ALL_LIMIT,
    offset: 0,
  });
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

// Per-unit sell-price history (one row per change, grouped by unit). Superset of
// the base-only listMedicinePrices — used by the medicine detail Price-history tab.
export function useMedicineUnitPricesQuery(medicineId: string, enabled = true) {
  return useQuery({
    queryKey: medicineKeys.unitPrices(medicineId),
    queryFn: async () => {
      const res = await medicineClient.listMedicineUnitPrices({ medicineId });
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
