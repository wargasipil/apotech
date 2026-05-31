import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { warehouseClient } from "../lib/clients";
import { ALL_LIMIT, DEFAULT_PAGE_SIZE } from "../lib/pagination";
import type {
  CreateWarehouseRequest,
  GrantWarehouseAccessRequest,
  SetDefaultWarehouseRequest,
  UpdateWarehouseRequest,
} from "../gen/warehouse_iface/v1/warehouse_pb";

export const warehouseKeys = {
  all: ["warehouses"] as const,
  list: (filters: object) => [...warehouseKeys.all, "list", filters] as const,
  user: (userId: string) => [...warehouseKeys.all, "user", userId] as const,
  detail: (id: string) => [...warehouseKeys.all, "detail", id] as const,
  users: (warehouseId: string) => [...warehouseKeys.all, "users", warehouseId] as const,
};

export type WarehousesQueryOpts = {
  includeInactive?: boolean;
  page?: number;
  pageSize?: number;
  query?: string;
};

// useWarehousesQuery is server-paginated like the other List* hooks: pass page
// + pageSize; the hook sends limit/offset and returns { rows, total }. For
// selector callers that need the whole list (Transfers From/To), use
// useAllWarehousesQuery below.
export function useWarehousesQuery(opts: WarehousesQueryOpts = {}) {
  const {
    includeInactive = false,
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
    query = "",
  } = opts;
  const q = useQuery({
    queryKey: warehouseKeys.list({ includeInactive, page, pageSize, query }),
    queryFn: async () => {
      const res = await warehouseClient.listWarehouses({
        includeInactive,
        limit: pageSize,
        offset: page * pageSize,
        query,
      });
      return { rows: res.warehouses, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// useAllWarehousesQuery is the "load every warehouse for a selector" helper —
// mirrors useAllMedicinesQuery / useAllCustomersQuery. Capped at ALL_LIMIT
// (1000), which is far above any realistic warehouse count.
export function useAllWarehousesQuery(includeInactive = false) {
  return useWarehousesQuery({ includeInactive, pageSize: ALL_LIMIT });
}

export function useMyWarehousesQuery() {
  return useQuery({
    queryKey: warehouseKeys.user("self"),
    queryFn: async () => warehouseClient.listUserWarehouses({ userId: "" }),
  });
}

// useWarehouseQuery — single warehouse by id; drives WarehouseDetail.
export function useWarehouseQuery(id: string) {
  return useQuery({
    queryKey: warehouseKeys.detail(id),
    queryFn: async () => {
      const res = await warehouseClient.getWarehouse({ id });
      return res.warehouse;
    },
    enabled: !!id,
  });
}

// useWarehouseUsersQuery — users with access to a warehouse (OWNER-only).
export function useWarehouseUsersQuery(warehouseId: string) {
  return useQuery({
    queryKey: warehouseKeys.users(warehouseId),
    queryFn: async () => {
      const res = await warehouseClient.listWarehouseUsers({ warehouseId });
      return [...res.users];
    },
    enabled: !!warehouseId,
  });
}

// Imperative search over the caller's accessible warehouses — backend ILIKEs
// code/name. Used by the TopBar warehouse popup so each keystroke (debounced)
// hits the backend. Mirrors the searchSuppliers / searchMedicines pattern.
export async function searchMyWarehouses(query: string) {
  const res = await warehouseClient.listUserWarehouses({ userId: "", query });
  return [...res.warehouses];
}

export function useCreateWarehouseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateWarehouseRequest>) => warehouseClient.createWarehouse(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: warehouseKeys.all }),
  });
}

export function useUpdateWarehouseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateWarehouseRequest>) => warehouseClient.updateWarehouse(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: warehouseKeys.all }),
  });
}

export function useArchiveWarehouseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => warehouseClient.archiveWarehouse({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: warehouseKeys.all }),
  });
}

export function useGrantWarehouseAccessMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<GrantWarehouseAccessRequest>) =>
      warehouseClient.grantWarehouseAccess(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: warehouseKeys.all }),
  });
}

export function useRevokeWarehouseAccessMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { userId: string; warehouseId: string }) =>
      warehouseClient.revokeWarehouseAccess(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: warehouseKeys.all }),
  });
}

export function useSetDefaultWarehouseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetDefaultWarehouseRequest>) =>
      warehouseClient.setDefaultWarehouse(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: warehouseKeys.all }),
  });
}

export function useSetGlobalDefaultWarehouseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (warehouseId: string) =>
      warehouseClient.setGlobalDefaultWarehouse({ warehouseId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: warehouseKeys.all }),
  });
}
