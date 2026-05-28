import { MetricType } from "../gen/analytics_iface/v1/analytics_pb";

// Field id convention: "<group>.<field>" strings. The frontend stores a
// Set<string> of visible fields; the backend still consumes group-level
// MetricType[]. This module is the translation layer.

export const ORDER_FIELD_IDS = [
  "order.terjual",
  "order.hpp",
  "order.profit",
] as const;

export const STOCK_FIELD_IDS = ["stock.ready", "stock.ongoing"] as const;

export const ALL_FIELD_IDS = [...ORDER_FIELD_IDS, ...STOCK_FIELD_IDS];

// Default selection per dimension.
export const DEFAULT_DAILY_FIELDS = new Set<string>(ALL_FIELD_IDS);
export const DEFAULT_PRODUCT_FIELDS = new Set<string>(ALL_FIELD_IDS);
export const DEFAULT_USER_FIELDS = new Set<string>(ORDER_FIELD_IDS);

// fieldsToMetricTypes derives the backend's metric_types list from the
// frontend's per-field selection: a group is included iff at least one of its
// fields is selected. Order matters for the API but Dim-pages each pass a
// stable order; sort here to keep request payloads deterministic for the
// query key.
export function fieldsToMetricTypes(fields: Set<string>): MetricType[] {
  const types: MetricType[] = [];
  if (ORDER_FIELD_IDS.some((id) => fields.has(id))) types.push(MetricType.ORDER);
  if (STOCK_FIELD_IDS.some((id) => fields.has(id))) types.push(MetricType.STOCK);
  return types;
}
