import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { warehouseClient } from "../lib/clients";
import type {
  CreateWarehouseRequest,
  GrantWarehouseAccessRequest,
  ListWarehousesRequest,
  SetDefaultWarehouseRequest,
  UpdateWarehouseRequest,
} from "../gen/warehouse_iface/v1/warehouse_pb";

export const warehouseKeys = {
  all: ["warehouses"] as const,
  list: (filters: object) => [...warehouseKeys.all, "list", filters] as const,
  user: (userId: string) => [...warehouseKeys.all, "user", userId] as const,
};

export function useWarehousesQuery(req: PartialMessage<ListWarehousesRequest> = {}) {
  return useQuery({
    queryKey: warehouseKeys.list(req),
    queryFn: async () => {
      const res = await warehouseClient.listWarehouses(req);
      return res.warehouses;
    },
  });
}

export function useMyWarehousesQuery() {
  return useQuery({
    queryKey: warehouseKeys.user("self"),
    queryFn: async () => warehouseClient.listUserWarehouses({ userId: "" }),
  });
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

export function useSetDefaultWarehouseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetDefaultWarehouseRequest>) =>
      warehouseClient.setDefaultWarehouse(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: warehouseKeys.all }),
  });
}
