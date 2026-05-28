import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import {
  batchClient,
  customerClient,
  medicineClient,
  supplierClient,
  userClient,
} from "../lib/clients";
import type { MedicineRef } from "../gen/inventory_iface/v1/medicine_pb";
import type { SupplierRef } from "../gen/inventory_iface/v1/supplier_pb";
import type { BatchRef } from "../gen/inventory_iface/v1/batch_pb";
import type { CustomerRef } from "../gen/customer_iface/v1/customer_pb";
import type { UserRef } from "../gen/user_iface/v1/users_pb";

// Resolve-by-IDs name lookups (HARD RULE). A page collects the referenced IDs on
// its current page and calls Resolve<Domain>(ids) to build an id → ref map for
// display. This is bounded by page size — NOT a full-list useAll*Query() preload.
// While resolving / for a genuinely missing id, render `map.get(id)?.name ?? "—"`
// (never a raw ID).

// cleanIds trims blanks, dedupes, and sorts so the query key is stable regardless
// of the order the page collected the ids in (so [a,b] and [b,a] share a cache entry).
function cleanIds(ids: string[]): string[] {
  return Array.from(new Set(ids.filter((id) => !!id))).sort();
}

function useRefs<T extends { id: string }>(
  domain: string,
  ids: string[],
  fetcher: (ids: string[]) => Promise<T[]>,
): Map<string, T> {
  const cleaned = useMemo(() => cleanIds(ids), [ids]);
  const q = useQuery({
    queryKey: [domain, cleaned],
    queryFn: () => fetcher(cleaned),
    enabled: cleaned.length > 0,
    staleTime: 60_000,
  });
  return useMemo(() => {
    const m = new Map<string, T>();
    for (const r of q.data ?? []) m.set(r.id, r);
    return m;
  }, [q.data]);
}

export function useMedicineRefs(ids: string[]): Map<string, MedicineRef> {
  return useRefs("medicineRefs", ids, async (i) => {
    const res = await medicineClient.resolveMedicines({ ids: i });
    return res.medicines;
  });
}

export function useSupplierRefs(ids: string[]): Map<string, SupplierRef> {
  return useRefs("supplierRefs", ids, async (i) => {
    const res = await supplierClient.resolveSuppliers({ ids: i });
    return res.suppliers;
  });
}

export function useCustomerRefs(ids: string[]): Map<string, CustomerRef> {
  return useRefs("customerRefs", ids, async (i) => {
    const res = await customerClient.resolveCustomers({ ids: i });
    return res.customers;
  });
}

export function useBatchRefs(ids: string[]): Map<string, BatchRef> {
  return useRefs("batchRefs", ids, async (i) => {
    const res = await batchClient.resolveBatches({ ids: i });
    return res.batches;
  });
}

export function useUserRefs(ids: string[]): Map<string, UserRef> {
  return useRefs("userRefs", ids, async (i) => {
    const res = await userClient.resolveUsers({ ids: i });
    return res.users;
  });
}

// Imperative resolve-by-IDs for one-shot needs (e.g. CSV export): dedupe, chunk
// to the backend's 500-id cap, and merge into an id → ref Map. Not hooks.
function chunk<T>(arr: T[], size: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < arr.length; i += size) out.push(arr.slice(i, i + size));
  return out;
}

export async function resolveSupplierMap(ids: string[]): Promise<Map<string, SupplierRef>> {
  const map = new Map<string, SupplierRef>();
  for (const group of chunk(cleanIds(ids), 500)) {
    const res = await supplierClient.resolveSuppliers({ ids: group });
    for (const r of res.suppliers) map.set(r.id, r);
  }
  return map;
}

export async function resolveBatchMap(ids: string[]): Promise<Map<string, BatchRef>> {
  const map = new Map<string, BatchRef>();
  for (const group of chunk(cleanIds(ids), 500)) {
    const res = await batchClient.resolveBatches({ ids: group });
    for (const r of res.batches) map.set(r.id, r);
  }
  return map;
}

export async function resolveUserMap(ids: string[]): Promise<Map<string, UserRef>> {
  const map = new Map<string, UserRef>();
  for (const group of chunk(cleanIds(ids), 500)) {
    const res = await userClient.resolveUsers({ ids: group });
    for (const r of res.users) map.set(r.id, r);
  }
  return map;
}
