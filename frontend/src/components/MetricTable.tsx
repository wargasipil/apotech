import { Box, HStack, Spinner, Table, Text } from "@chakra-ui/react";
import { ArrowDown, ArrowUp } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  MetricType,
  OrderMetricField,
  type MetricOrder,
  type MetricStock,
  Sort,
  SortDirection,
  StockMetricField,
} from "../gen/analytics_iface/v1/analytics_pb";
import { formatMoney } from "../lib/format";

// MetricTable renders a paginated metric grid. The page passes:
//   - `ids` in the order the server returned (already sorted)
//   - `order` + `stock` maps keyed by id (whichever was requested)
//   - `metricTypes` so the table shows only the requested column groups
//   - `labelById` Map<id, displayName>; missing -> "—"
//   - `sort` + `onSortChange` to drive backend-side sort via column headers
type Props = {
  ids: string[];
  order?: MetricOrder;
  stock?: MetricStock;
  metricTypes: MetricType[];
  // visibleFields drives per-column visibility (field ids like "order.terjual",
  // "stock.ready"). The set is independent of metricTypes — even if ORDER is in
  // metricTypes, individual order columns are hidden when their field id is
  // absent. When omitted, defaults to "every field in any requested metric".
  visibleFields?: Set<string>;
  labelById: Map<string, string>;
  sort?: Sort;
  // undefined = clear sort (revert to backend default = sort by dimension key)
  onSortChange?: (sort: Sort | undefined) => void;
  dimensionHeader: string;
  isLoading?: boolean;
};

export default function MetricTable({
  ids,
  order,
  stock,
  metricTypes,
  visibleFields,
  labelById,
  sort,
  onSortChange,
  dimensionHeader,
  isLoading,
}: Props) {
  const { t } = useTranslation();
  // Per-field visibility, gated by both visibleFields (UI choice) and
  // metricTypes (the group must actually be requested from the backend).
  const want = (groupOk: boolean, fieldId: string) =>
    groupOk && (visibleFields ? visibleFields.has(fieldId) : true);
  const hasOrderGroup = metricTypes.includes(MetricType.ORDER);
  const hasStockGroup = metricTypes.includes(MetricType.STOCK);
  const showTerjual = want(hasOrderGroup, "order.terjual");
  const showHpp     = want(hasOrderGroup, "order.hpp");
  const showProfit  = want(hasOrderGroup, "order.profit");
  const showReady   = want(hasStockGroup, "stock.ready");
  const showOngoing = want(hasStockGroup, "stock.ongoing");
  const orderCols = (showTerjual ? 1 : 0) + (showHpp ? 1 : 0) + (showProfit ? 1 : 0);
  const stockCols = (showReady ? 1 : 0) + (showOngoing ? 1 : 0);
  const hasOrder = orderCols > 0;
  const hasStock = stockCols > 0;
  const colSpan = 1 + orderCols + stockCols;

  // Column-header click toggles sort: unsorted -> DESC -> ASC -> unsorted.
  type ColField =
    | "order-terjual"
    | "order-hpp"
    | "order-profit"
    | "stock-ready"
    | "stock-ongoing";
  const sortFor = (
    field: ColField,
  ): { arrow: React.ReactNode; next: Sort | undefined; hasNext: boolean } => {
    const current = matchSort(sort, field);
    const arrow = current
      ? current === SortDirection.DESC
        ? <ArrowDown size={12} />
        : <ArrowUp size={12} />
      : null;
    return { arrow, next: nextSort(field, current), hasNext: true };
  };

  return (
    <Box overflowX="auto">
      <Table.Root size="sm" variant="line">
        <Table.Header>
          {(hasOrder || hasStock) && (
            <Table.Row bg="bg.muted">
              <Table.ColumnHeader rowSpan={2}>{dimensionHeader}</Table.ColumnHeader>
              {hasOrder && (
                <Table.ColumnHeader colSpan={orderCols} textAlign="center">
                  {t("analytics.metric.group.order")}
                </Table.ColumnHeader>
              )}
              {hasStock && (
                <Table.ColumnHeader colSpan={stockCols} textAlign="center">
                  {t("analytics.metric.group.stock")}
                </Table.ColumnHeader>
              )}
            </Table.Row>
          )}
          <Table.Row>
            {!hasOrder && !hasStock && (
              <Table.ColumnHeader>{dimensionHeader}</Table.ColumnHeader>
            )}
            {showTerjual && (
              <SortableHeader
                label={t("analytics.metric.order.terjual")}
                meta={sortFor("order-terjual")}
                onClick={onSortChange}
              />
            )}
            {showHpp && (
              <SortableHeader
                label={t("analytics.metric.order.hpp")}
                meta={sortFor("order-hpp")}
                onClick={onSortChange}
              />
            )}
            {showProfit && (
              <SortableHeader
                label={t("analytics.metric.order.profit")}
                meta={sortFor("order-profit")}
                onClick={onSortChange}
              />
            )}
            {showReady && (
              <SortableHeader
                label={t("analytics.metric.stock.ready")}
                meta={sortFor("stock-ready")}
                onClick={onSortChange}
              />
            )}
            {showOngoing && (
              <SortableHeader
                label={t("analytics.metric.stock.ongoing")}
                meta={sortFor("stock-ongoing")}
                onClick={onSortChange}
              />
            )}
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {isLoading && (
            <Table.Row>
              <Table.Cell colSpan={colSpan}>
                <Box p={6} textAlign="center">
                  <Spinner size="sm" />
                </Box>
              </Table.Cell>
            </Table.Row>
          )}
          {!isLoading &&
            ids.map((id) => {
              const o = order?.data[id];
              const s = stock?.data[id];
              const label = labelById.get(id) ?? id;
              return (
                <Table.Row key={id}>
                  <Table.Cell>{label}</Table.Cell>
                  {showTerjual && <Table.Cell>{formatMoney(Number(o?.terjual ?? 0n))}</Table.Cell>}
                  {showHpp && <Table.Cell>{formatMoney(Number(o?.hpp ?? 0n))}</Table.Cell>}
                  {showProfit && <Table.Cell>{formatMoney(Number(o?.profit ?? 0n))}</Table.Cell>}
                  {showReady && <Table.Cell>{String(s?.ready ?? 0n)}</Table.Cell>}
                  {showOngoing && <Table.Cell>{String(s?.ongoing ?? 0n)}</Table.Cell>}
                </Table.Row>
              );
            })}
          {!isLoading && ids.length === 0 && (
            <Table.Row>
              <Table.Cell colSpan={colSpan}>
                <Text color="fg.muted" textAlign="center" py={4}>
                  {t("common.noResults")}
                </Text>
              </Table.Cell>
            </Table.Row>
          )}
        </Table.Body>
      </Table.Root>
    </Box>
  );
}

// SortableHeader = clickable column header that cycles direction.
function SortableHeader({
  label,
  meta,
  onClick,
}: {
  label: string;
  meta: { arrow: React.ReactNode; next: Sort | undefined; hasNext: boolean };
  onClick?: (sort: Sort | undefined) => void;
}) {
  const clickable = !!onClick && meta.hasNext;
  return (
    <Table.ColumnHeader
      cursor={clickable ? "pointer" : "default"}
      onClick={clickable ? () => onClick?.(meta.next) : undefined}
    >
      <HStack gap={1}>
        <Text>{label}</Text>
        {meta.arrow}
      </HStack>
    </Table.ColumnHeader>
  );
}

// matchSort returns the current SortDirection for the column field, or null
// when the current sort doesn't reference this column.
function matchSort(
  sort: Sort | undefined,
  field: "order-terjual" | "order-hpp" | "order-profit" | "stock-ready" | "stock-ongoing",
): SortDirection | null {
  if (!sort || !sort.field) return null;
  switch (sort.field.case) {
    case "order": {
      const want =
        field === "order-terjual"
          ? OrderMetricField.TERJUAL
          : field === "order-hpp"
          ? OrderMetricField.HPP
          : field === "order-profit"
          ? OrderMetricField.PROFIT
          : null;
      return want !== null && sort.field.value === want ? sort.direction : null;
    }
    case "stock": {
      const want =
        field === "stock-ready"
          ? StockMetricField.READY
          : field === "stock-ongoing"
          ? StockMetricField.ONGOING
          : null;
      return want !== null && sort.field.value === want ? sort.direction : null;
    }
  }
  return null;
}

// nextSort cycles the sort direction for a column: unsorted -> DESC -> ASC ->
// unsorted (returns undefined to clear).
function nextSort(
  field: "order-terjual" | "order-hpp" | "order-profit" | "stock-ready" | "stock-ongoing",
  current: SortDirection | null,
): Sort | undefined {
  const newDir =
    current === null
      ? SortDirection.DESC
      : current === SortDirection.DESC
      ? SortDirection.ASC
      : null;
  if (newDir === null) {
    // Cycle back to "unsorted" — let the caller clear it.
    return undefined;
  }
  const isOrder = field.startsWith("order-");
  if (isOrder) {
    const orderField =
      field === "order-terjual"
        ? OrderMetricField.TERJUAL
        : field === "order-hpp"
        ? OrderMetricField.HPP
        : OrderMetricField.PROFIT;
    return new Sort({
      direction: newDir,
      field: { case: "order", value: orderField },
    });
  }
  const stockField =
    field === "stock-ready" ? StockMetricField.READY : StockMetricField.ONGOING;
  return new Sort({
    direction: newDir,
    field: { case: "stock", value: stockField },
  });
}
