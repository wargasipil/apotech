import { Box, Grid, Heading, Spinner, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import {
  MetricType,
  type MetricOrder,
  type MetricStock,
} from "../gen/analytics_iface/v1/analytics_pb";
import { formatMoney } from "../lib/format";

// MetricGraphs — one LineChart per active metric field (terjual/hpp/profit
// when ORDER requested + ready/ongoing when STOCK requested). X = the
// dimension key (the day string for Daily), Y = the field's value pulled out
// of the corresponding map. Daily-only per the plan.
type Props = {
  ids: string[];
  order?: MetricOrder;
  stock?: MetricStock;
  metricTypes: MetricType[];
  // visibleFields: per-field visibility (e.g. "order.terjual"). Charts for
  // un-listed fields are skipped. When omitted, every field in the requested
  // metric groups is shown.
  visibleFields?: Set<string>;
  isLoading?: boolean;
};

export default function MetricGraphs({
  ids,
  order,
  stock,
  metricTypes,
  visibleFields,
  isLoading,
}: Props) {
  const { t } = useTranslation();
  const want = (groupOk: boolean, fieldId: string) =>
    groupOk && (visibleFields ? visibleFields.has(fieldId) : true);
  const hasOrderGroup = metricTypes.includes(MetricType.ORDER);
  const hasStockGroup = metricTypes.includes(MetricType.STOCK);
  const showTerjual = want(hasOrderGroup, "order.terjual");
  const showHpp     = want(hasOrderGroup, "order.hpp");
  const showProfit  = want(hasOrderGroup, "order.profit");
  const showReady   = want(hasStockGroup, "stock.ready");
  const showOngoing = want(hasStockGroup, "stock.ongoing");

  if (isLoading) {
    return (
      <Box p={6} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  return (
    <Grid templateColumns={{ base: "1fr", md: "repeat(2, 1fr)" }} gap={4}>
      {showTerjual && (
        <Chart
          title={t("analytics.metric.order.terjual")}
          data={ids.map((id) => ({ key: id, value: Number(order?.data[id]?.terjual ?? 0n) }))}
          money
        />
      )}
      {showHpp && (
        <Chart
          title={t("analytics.metric.order.hpp")}
          data={ids.map((id) => ({ key: id, value: Number(order?.data[id]?.hpp ?? 0n) }))}
          money
        />
      )}
      {showProfit && (
        <Chart
          title={t("analytics.metric.order.profit")}
          data={ids.map((id) => ({ key: id, value: Number(order?.data[id]?.profit ?? 0n) }))}
          money
        />
      )}
      {showReady && (
        <Chart
          title={t("analytics.metric.stock.ready")}
          data={ids.map((id) => ({ key: id, value: Number(stock?.data[id]?.ready ?? 0n) }))}
        />
      )}
      {showOngoing && (
        <Chart
          title={t("analytics.metric.stock.ongoing")}
          data={ids.map((id) => ({ key: id, value: Number(stock?.data[id]?.ongoing ?? 0n) }))}
        />
      )}
    </Grid>
  );
}

function Chart({
  title,
  data,
  money,
}: {
  title: string;
  data: { key: string; value: number }[];
  money?: boolean;
}) {
  return (
    <Stack
      gap={2}
      bg="bg.subtle"
      borderWidth="1px"
      borderRadius="lg"
      p={3}
      minH="220px"
    >
      <Heading size="xs" color="fg.muted">
        {title}
      </Heading>
      <Box h="180px">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--chakra-colors-border)" />
            <XAxis dataKey="key" fontSize={10} />
            <YAxis
              fontSize={10}
              tickFormatter={money ? (v) => formatMoney(v).replace("Rp", "") : undefined}
            />
            <Tooltip formatter={(v: number) => (money ? formatMoney(v) : v)} />
            <Line type="monotone" dataKey="value" stroke="#3B82F6" strokeWidth={2} dot={false} />
          </LineChart>
        </ResponsiveContainer>
      </Box>
    </Stack>
  );
}
