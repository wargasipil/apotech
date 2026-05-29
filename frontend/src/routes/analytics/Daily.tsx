import { Box, HStack, Heading, Stack, Tabs } from "@chakra-ui/react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import ColumnsPopover, { type GroupSpec } from "../../components/ColumnsPopover";
import DateRangeFilter, {
  resolveRange,
  type DateRange,
} from "../../components/DateRangeFilter";
import EnumSelect from "../../components/EnumSelect";
import MetricGraphs from "../../components/MetricGraphs";
import MetricTable from "../../components/MetricTable";
import { Granularity, Sort, SortDirection } from "../../gen/analytics_iface/v1/analytics_pb";
import {
  DEFAULT_DAILY_FIELDS,
  fieldsToMetricTypes,
} from "../../lib/analyticsFields";
import { useDailyMetricQuery } from "../../queries/analytics";

// Daily menu — one row per day/week/month bucket. Two tabs: Table + Graphs.
export default function Daily() {
  const { t } = useTranslation();
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const [granularity, setGranularity] = useState<Granularity>(Granularity.DAY);
  const [visibleFields, setVisibleFields] = useState<Set<string>>(
    () => new Set(DEFAULT_DAILY_FIELDS),
  );
  // Default to newest-day-first; backend honors direction even when the sort
  // field is unset (treated as "sort by dimension key"). Clicking a metric
  // column header still overrides this with that metric's sort.
  const [sort, setSort] = useState<Sort | undefined>(
    () => new Sort({ direction: SortDirection.DESC }),
  );

  const metricTypes = useMemo(
    () => fieldsToMetricTypes(visibleFields),
    [visibleFields],
  );

  const q = useDailyMetricQuery({
    metricTypes,
    filter: { fromUnix: BigInt(range.fromUnix), toUnix: BigInt(range.toUnix) },
    sort,
    granularity,
  });

  // Day strings ARE the labels — no resolve needed for Daily.
  const labelById = useMemo(() => {
    const m = new Map<string, string>();
    for (const d of q.data?.days ?? []) m.set(d, d);
    return m;
  }, [q.data?.days]);

  return (
    <Stack gap={4}>
      <HStack justify="space-between" wrap="wrap" gap={3}>
        <Heading size="sm">{t("analytics.menu.daily")}</Heading>
        <HStack gap={2} wrap="wrap">
          <ColumnsPopover
            value={visibleFields}
            onChange={(next) => {
              setVisibleFields(next);
              clearSortIfHidden(sort, next, setSort);
            }}
            groups={defaultGroups(t)}
            defaults={DEFAULT_DAILY_FIELDS}
          />
          <EnumSelect
            size="sm"
            width="140px"
            value={String(granularity)}
            onChange={(v) => setGranularity(Number(v) as Granularity)}
            items={[
              { value: String(Granularity.DAY), label: t("analytics.granularity.day") },
              { value: String(Granularity.WEEK), label: t("analytics.granularity.week") },
              { value: String(Granularity.MONTH), label: t("analytics.granularity.month") },
            ]}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
          />
          <DateRangeFilter value={range} onChange={setRange} />
        </HStack>
      </HStack>

      <Tabs.Root defaultValue="table" variant="line">
        <Tabs.List>
          <Tabs.Trigger value="table">{t("analytics.tab.table")}</Tabs.Trigger>
          <Tabs.Trigger value="graph">{t("analytics.tab.graph")}</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="table">
          <Box pt={3}>
            <MetricTable
              ids={q.data?.days ?? []}
              order={q.data?.order}
              stock={q.data?.stock}
              metricTypes={metricTypes}
              visibleFields={visibleFields}
              labelById={labelById}
              sort={sort}
              onSortChange={setSort}
              dimensionHeader={t("analytics.menu.daily")}
              isLoading={q.isLoading}
            />
          </Box>
        </Tabs.Content>
        <Tabs.Content value="graph">
          <Box pt={3}>
            <MetricGraphs
              ids={q.data?.days ?? []}
              order={q.data?.order}
              stock={q.data?.stock}
              metricTypes={metricTypes}
              visibleFields={visibleFields}
              isLoading={q.isLoading}
            />
          </Box>
        </Tabs.Content>
      </Tabs.Root>
    </Stack>
  );
}

// defaultGroups returns the metric-group spec used by all 3 analytics pages.
// Pages override the `disabled` flag (User disables Stock). `productExtras`
// appends the Product-only fields (last_order / avg_sold / last_restock /
// expiring) — Daily and User omit them since the backend zeroes them outside
// ProductMetric.
export function defaultGroups(
  t: (k: string) => string,
  opts: { disableStock?: boolean; disableStockReason?: string; productExtras?: boolean } = {},
): GroupSpec[] {
  const orderFields = [
    { id: "order.terjual", label: t("analytics.metric.order.terjual") },
    { id: "order.hpp",     label: t("analytics.metric.order.hpp") },
    { id: "order.profit",  label: t("analytics.metric.order.profit") },
  ];
  const stockFields = [
    { id: "stock.ready",   label: t("analytics.metric.stock.ready") },
    { id: "stock.ongoing", label: t("analytics.metric.stock.ongoing") },
  ];
  if (opts.productExtras) {
    orderFields.push(
      { id: "order.lastOrder", label: t("analytics.metric.order.lastOrder") },
      { id: "order.avgSold",   label: t("analytics.metric.order.avgSold") },
    );
    stockFields.push(
      { id: "stock.lastRestock", label: t("analytics.metric.stock.lastRestock") },
      { id: "stock.expiring",    label: t("analytics.metric.stock.expiring") },
    );
  }
  return [
    {
      id: "order",
      label: t("analytics.metric.group.order"),
      fields: orderFields,
    },
    {
      id: "stock",
      label: t("analytics.metric.group.stock"),
      fields: stockFields,
      disabled: opts.disableStock,
      disabledReason: opts.disableStockReason,
    },
  ];
}

// clearSortIfHidden clears the sort state when the user un-checks the column
// it currently references — otherwise the sort indicator becomes invisible.
export function clearSortIfHidden(
  sort: Sort | undefined,
  visible: Set<string>,
  setSort: (s: Sort | undefined) => void,
): void {
  if (!sort || !sort.field) return;
  switch (sort.field.case) {
    case "order": {
      const map: Record<number, string> = {
        1: "order.terjual",   // TERJUAL
        2: "order.hpp",       // HPP
        3: "order.profit",    // PROFIT
        4: "order.lastOrder", // LAST_ORDER
        5: "order.avgSold",   // AVG_SOLD
      };
      const id = map[sort.field.value as number];
      if (id && !visible.has(id)) setSort(undefined);
      break;
    }
    case "stock": {
      const map: Record<number, string> = {
        1: "stock.ready",       // READY
        2: "stock.ongoing",     // ONGOING
        3: "stock.lastRestock", // LAST_RESTOCK
        4: "stock.expiring",    // EXPIRING
      };
      const id = map[sort.field.value as number];
      if (id && !visible.has(id)) setSort(undefined);
      break;
    }
  }
}

