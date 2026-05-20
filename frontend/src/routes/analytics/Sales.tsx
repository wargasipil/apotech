import { useState } from "react";
import {
  Box,
  Button,
  Grid,
  HStack,
  Heading,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";

import EnumSelect from "../../components/EnumSelect";
import { Download } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import DateRangeFilter, {
  resolveRange,
  type DateRange,
} from "../../components/DateRangeFilter";
import HourHeatmap from "../../components/HourHeatmap";
import { Granularity, SortMetric } from "../../gen/analytics_iface/v1/sales_pb";
import { downloadCsv } from "../../lib/csv";
import { formatMoney } from "../../lib/format";
import {
  useHourOfDayQuery,
  usePaymentMixQuery,
  useRevenueTrendQuery,
  useSalesByCashierQuery,
  useTopSellersQuery,
} from "../../queries/analytics";

const PIE_COLORS = ["#3B82F6", "#10B981", "#F59E0B", "#EF4444", "#8B5CF6", "#0EA5E9"];

export default function SalesAnalytics() {
  const { t } = useTranslation();
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const [topMetric, setTopMetric] = useState<SortMetric>(SortMetric.REVENUE);

  const revenueQ = useRevenueTrendQuery({
    fromUnix: BigInt(range.fromUnix),
    toUnix: BigInt(range.toUnix),
    granularity: Granularity.DAY,
  });
  const topQ = useTopSellersQuery({
    fromUnix: BigInt(range.fromUnix),
    toUnix: BigInt(range.toUnix),
    limit: 10,
    metric: topMetric,
  });
  const mixQ = usePaymentMixQuery({
    fromUnix: BigInt(range.fromUnix),
    toUnix: BigInt(range.toUnix),
  });
  const cashierQ = useSalesByCashierQuery({
    fromUnix: BigInt(range.fromUnix),
    toUnix: BigInt(range.toUnix),
  });
  const hourQ = useHourOfDayQuery({
    fromUnix: BigInt(range.fromUnix),
    toUnix: BigInt(range.toUnix),
  });

  const revenueData = (revenueQ.data?.points ?? []).map((p) => ({
    bucket: p.bucket,
    revenue: Number(p.revenue),
    sale_count: Number(p.saleCount),
  }));
  const topData = (topQ.data?.items ?? []).map((i) => ({
    medicine: i.medicineName,
    qty: Number(i.qty),
    revenue: Number(i.revenue),
  }));
  const mixData = (mixQ.data?.slices ?? []).map((s) => ({
    payment: s.paymentSource || "—",
    revenue: Number(s.revenue),
    sale_count: Number(s.saleCount),
  }));

  return (
    <Stack gap={6}>
      <HStack justify="space-between" wrap="wrap" gap={2}>
        <Heading size="sm">{t("analytics.tabs.sales")}</Heading>
        <DateRangeFilter value={range} onChange={setRange} />
      </HStack>

      <Section title={t("analytics.sales.revenueTrend")}>
        {revenueQ.isLoading ? (
          <CenterSpinner />
        ) : (
          <Box h="280px">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={revenueData}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--chakra-colors-border)" />
                <XAxis dataKey="bucket" fontSize={11} />
                <YAxis fontSize={11} tickFormatter={(v) => formatMoney(v).replace("Rp", "")} />
                <Tooltip formatter={(v: number) => formatMoney(v)} />
                <Line type="monotone" dataKey="revenue" stroke="#3B82F6" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </Box>
        )}
      </Section>

      <Grid templateColumns={{ base: "1fr", lg: "2fr 1fr" }} gap={6}>
        <Section
          title={t("analytics.sales.topSellers")}
          actions={
            <HStack gap={2}>
              <EnumSelect
                size="sm"
                width="140px"
                value={String(topMetric)}
                onChange={(v) => setTopMetric(Number(v) as SortMetric)}
                items={[
                  { value: String(SortMetric.REVENUE), label: t("analytics.sales.revenue") },
                  { value: String(SortMetric.QTY), label: t("analytics.sales.qty") },
                ]}
                itemToString={(o) => o.label}
                itemToValue={(o) => o.value}
              />
              <Button
                size="xs"
                variant="ghost"
                onClick={() =>
                  downloadCsv("top-sellers.csv", topData, [
                    { key: "medicine", header: "Medicine" },
                    { key: "qty", header: "Qty" },
                    { key: "revenue", header: "Revenue (IDR)" },
                  ])
                }
              >
                <Download size={14} />
                {t("analytics.exportCsv")}
              </Button>
            </HStack>
          }
        >
          {topQ.isLoading ? (
            <CenterSpinner />
          ) : (
            <Box h="280px">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={topData} layout="vertical">
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--chakra-colors-border)" />
                  <XAxis type="number" fontSize={11} />
                  <YAxis dataKey="medicine" type="category" fontSize={11} width={120} />
                  <Tooltip
                    formatter={(v: number, key: string) =>
                      key === "revenue" ? formatMoney(v) : v
                    }
                  />
                  <Bar
                    dataKey={topMetric === SortMetric.REVENUE ? "revenue" : "qty"}
                    fill="#3B82F6"
                  />
                </BarChart>
              </ResponsiveContainer>
            </Box>
          )}
        </Section>

        <Section title={t("analytics.sales.paymentMix")}>
          {mixQ.isLoading ? (
            <CenterSpinner />
          ) : (
            <Box h="280px">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Tooltip formatter={(v: number) => formatMoney(v)} />
                  <Pie
                    data={mixData}
                    dataKey="revenue"
                    nameKey="payment"
                    cx="50%"
                    cy="50%"
                    outerRadius={90}
                    label={(p: { payment?: string }) => p.payment ?? ""}
                  >
                    {mixData.map((_, i) => (
                      <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />
                    ))}
                  </Pie>
                </PieChart>
              </ResponsiveContainer>
            </Box>
          )}
        </Section>
      </Grid>

      <Section title={t("analytics.sales.hourHeatmap")}>
        {hourQ.isLoading ? (
          <CenterSpinner />
        ) : (
          <HourHeatmap cells={hourQ.data?.cells ?? []} />
        )}
      </Section>

      <Section
        title={t("analytics.sales.byCashier")}
        actions={
          <Button
            size="xs"
            variant="ghost"
            onClick={() =>
              downloadCsv(
                "sales-by-cashier.csv",
                (cashierQ.data?.items ?? []).map((c) => ({
                  cashier: c.userName || c.userEmail,
                  revenue: Number(c.revenue),
                  sale_count: Number(c.saleCount),
                })),
                [
                  { key: "cashier", header: "Cashier" },
                  { key: "sale_count", header: "Sales" },
                  { key: "revenue", header: "Revenue (IDR)" },
                ],
              )
            }
          >
            <Download size={14} />
            {t("analytics.exportCsv")}
          </Button>
        }
      >
        {cashierQ.isLoading ? (
          <CenterSpinner />
        ) : (
          <Table.Root size="sm">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>{t("analytics.sales.cashier")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("analytics.sales.count")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("analytics.sales.revenue")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {(cashierQ.data?.items ?? []).map((c) => (
                <Table.Row key={c.userId}>
                  <Table.Cell>{c.userName || c.userEmail}</Table.Cell>
                  <Table.Cell>{String(c.saleCount)}</Table.Cell>
                  <Table.Cell>{formatMoney(Number(c.revenue))}</Table.Cell>
                </Table.Row>
              ))}
              {(cashierQ.data?.items?.length ?? 0) === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={3}>
                    <Text color="fg.muted" textAlign="center" py={4}>
                      —
                    </Text>
                  </Table.Cell>
                </Table.Row>
              )}
            </Table.Body>
          </Table.Root>
        )}
      </Section>
    </Stack>
  );
}

function Section({
  title,
  actions,
  children,
}: {
  title: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
      <HStack justify="space-between" mb={3}>
        <Heading size="sm">{title}</Heading>
        {actions}
      </HStack>
      {children}
    </Box>
  );
}

function CenterSpinner() {
  return (
    <Box p={6} textAlign="center">
      <Spinner />
    </Box>
  );
}
