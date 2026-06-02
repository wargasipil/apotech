import type { MedicineUnit } from "../gen/inventory_iface/v1/medicine_pb";
import type { UnitBase } from "../gen/unit_iface/v1/unit_pb";

// Per-base preference: baseName → derivative name ("" = base / raw count).
export type StockUnitsByBase = Record<string, string>;

// formatStock renders a base-unit qty into a human-readable string. The
// preference is per-base — look up by the row's own base unit name. When the
// preference picks a derivative that the medicine defines, render
// `floor(qty / factor) + name`. Otherwise (no preference, or medicine lacks
// that unit) → base fallback `qty + baseUnitName`.
export function formatStock(
  qty: bigint,
  units: MedicineUnit[],
  baseUnitName: string,
  byBase: StockUnitsByBase,
): string {
  const baseLabel = baseUnitName || "";
  const mode = (baseUnitName && byBase[baseUnitName]) || "";
  if (!mode) return baseLabel ? `${qty} ${baseLabel}` : qty.toString();
  const u = units.find((x) => x.active && x.name === mode);
  if (!u || u.factor < 1n) return baseLabel ? `${qty} ${baseLabel}` : qty.toString();
  const whole = qty / u.factor;
  return `${whole} ${u.name}`;
}

// A group of options for a single base unit: the base name + its available
// derivative names (sorted; deduplicated by sort_order then name).
export type StockUnitGroup = { baseName: string; derivatives: string[] };

// unitGroupsFromCatalog builds one group per base unit. The catalog drives
// the canonical list (so options exist even when the current page has no
// medicine with that derivative). Derivative names seen on the current page
// but missing from the catalog are added in (legacy / pre-catalog units).
export function unitGroupsFromCatalog(
  catalog: UnitBase[],
  pageRows: Array<{ unit: string; units: MedicineUnit[] }>,
): StockUnitGroup[] {
  const groups = new Map<string, Map<string, number>>(); // baseName -> (derivName -> sort_order)
  for (const b of catalog) {
    if (!b.active) continue;
    const m = groups.get(b.name) ?? new Map<string, number>();
    for (const d of b.derivatives) {
      if (!d.active) continue;
      m.set(d.name, d.sortOrder);
    }
    groups.set(b.name, m);
  }
  // Augment with page-discovered derivatives so legacy medicines still get a
  // working dropdown even when their base/derivative isn't in the catalog.
  for (const row of pageRows) {
    const baseName = row.unit;
    if (!baseName) continue;
    const m = groups.get(baseName) ?? new Map<string, number>();
    for (const u of row.units) {
      if (!u.active || u.isBase) continue;
      if (!m.has(u.name)) m.set(u.name, u.sortOrder);
    }
    groups.set(baseName, m);
  }
  return [...groups.entries()]
    .sort((a, b) => a[0].localeCompare(b[0]))
    .map(([baseName, derivs]) => ({
      baseName,
      derivatives: [...derivs.entries()]
        .sort((a, b) => a[1] - b[1] || a[0].localeCompare(b[0]))
        .map(([n]) => n),
    }));
}
