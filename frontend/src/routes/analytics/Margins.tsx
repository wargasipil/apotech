import { useState } from "react";
import {
  Box,
  Button,
  HStack,
  Heading,
  NativeSelect,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Download } from "lucide-react";
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

import DateRangeFilter, {
  resolveRange,
  type DateRange,
} from "../../components/DateRangeFilter";
import { downloadCsv } from "../../lib/csv";
import { formatMoney } from "../../lib/format";
import { useSuppliersQuery } from "../../queries/suppliers";
import {
  useMarginPerMedicineQuery,
  useSupplierCostTrendQuery,
  useTopMarginQuery,
} from "../../queries/analytics";

export default function MarginsAnalytics() {
  const { t } = useTranslation();
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const [supplierId, setSupplierId] = useState("");

  const perMedicineQ = useMarginPerMedicineQuery({
    fromUnix: BigInt(range.fromUnix),
    toUnix: BigInt(range.toUnix),
    limit: 50,
  });
  const topQ = useTopMarginQuery({
    fromUnix: BigInt(range.fromUnix),
    toUnix: BigInt(range.toUnix),
    limit: 10,
  });
  const suppliersQ = useSuppliersQuery(false);
  const trendQ = useSupplierCostTrendQuery({ supplierId }, !!supplierId);

  return (
    <Stack gap={6}>
      <HStack justify="space-between" wrap="wrap" gap={2}>
        <Heading size="sm">{t("analytics.tabs.margins")}</Heading>
        <DateRangeFilter value={range} onChange={setRange} />
      </HStack>

      <Section
        title={t("analytics.margin.topMargin")}
        actions={
          <Button
            size="xs"
            variant="ghost"
            onClick={() =>
              downloadCsv(
                "top-margin.csv",
                (topQ.data?.rows ?? []).map((r) => ({
                  sku: r.sku,
                  medicine: r.medicineName,
                  revenue: Number(r.revenue),
                  cogs: Number(r.cogs),
                  gross: Number(r.grossMargin),
                  margin_pct: (r.marginPct * 100).toFixed(1) + "%",
                })),
                [
                  { key: "sku", header: "SKU" },
                  { key: "medicine", header: "Medicine" },
                  { key: "revenue", header: "Revenue (IDR)" },
                  { key: "cogs", header: "COGS (IDR)" },
                  { key: "gross", header: "Gross margin (IDR)" },
                  { key: "margin_pct", header: "Margin %" },
                ],
              )
            }
          >
            <Download size={14} />
            {t("analytics.exportCsv")}
          </Button>
        }
      >
        {topQ.isLoading ? (
          <CenterSpinner />
        ) : (
          <MarginTable rows={topQ.data?.rows ?? []} />
        )}
      </Section>

      <Section
        title={t("analytics.margin.perMedicine")}
        actions={
          <Button
            size="xs"
            variant="ghost"
            onClick={() =>
              downloadCsv(
                "margin-per-medicine.csv",
                (perMedicineQ.data?.rows ?? []).map((r) => ({
                  sku: r.sku,
                  medicine: r.medicineName,
                  revenue: Number(r.revenue),
                  cogs: Number(r.cogs),
                  gross: Number(r.grossMargin),
                  margin_pct: (r.marginPct * 100).toFixed(1) + "%",
                })),
                [
                  { key: "sku", header: "SKU" },
                  { key: "medicine", header: "Medicine" },
                  { key: "revenue", header: "Revenue (IDR)" },
                  { key: "cogs", header: "COGS (IDR)" },
                  { key: "gross", header: "Gross margin (IDR)" },
                  { key: "margin_pct", header: "Margin %" },
                ],
              )
            }
          >
            <Download size={14} />
            {t("analytics.exportCsv")}
          </Button>
        }
      >
        {perMedicineQ.isLoading ? (
          <CenterSpinner />
        ) : (
          <MarginTable rows={perMedicineQ.data?.rows ?? []} />
        )}
      </Section>

      <Section
        title={t("analytics.margin.supplierCostTrend")}
        actions={
          <NativeSelect.Root size="sm" width="auto">
            <NativeSelect.Field
              value={supplierId}
              onChange={(e) => setSupplierId(e.target.value)}
            >
              <option value="">{t("analytics.margin.selectSupplier")}</option>
              {(suppliersQ.data ?? []).map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </NativeSelect.Field>
            <NativeSelect.Indicator />
          </NativeSelect.Root>
        }
      >
        {!supplierId ? (
          <Text color="fg.muted" fontSize="sm" textAlign="center" py={6}>
            {t("analytics.margin.selectSupplier")}
          </Text>
        ) : trendQ.isLoading ? (
          <CenterSpinner />
        ) : (
          <Box h="280px">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                data={(trendQ.data?.points ?? []).map((p) => ({
                  date: p.receivedAt,
                  cost: Number(p.costPrice),
                  medicine: p.medicineName,
                  batch: p.batchNumber,
                }))}
              >
                <CartesianGrid strokeDasharray="3 3" stroke="var(--chakra-colors-border)" />
                <XAxis dataKey="date" fontSize={11} />
                <YAxis fontSize={11} tickFormatter={(v) => formatMoney(v).replace("Rp", "")} />
                <Tooltip
                  formatter={(v: number) => formatMoney(v)}
                  labelFormatter={(_l, payload) => {
                    const p = payload?.[0]?.payload as
                      | { medicine?: string; batch?: string; date?: string }
                      | undefined;
                    return p
                      ? `${p.date} · ${p.medicine}${p.batch ? ` (#${p.batch})` : ""}`
                      : "";
                  }}
                />
                <Line type="monotone" dataKey="cost" stroke="#3B82F6" strokeWidth={2} />
              </LineChart>
            </ResponsiveContainer>
          </Box>
        )}
      </Section>
    </Stack>
  );
}

function MarginTable({
  rows,
}: {
  rows: {
    medicineId: string;
    medicineName: string;
    sku: string;
    revenue: bigint;
    cogs: bigint;
    grossMargin: bigint;
    marginPct: number;
  }[];
}) {
  const { t } = useTranslation();
  return (
    <Table.Root size="sm">
      <Table.Header>
        <Table.Row>
          <Table.ColumnHeader>SKU</Table.ColumnHeader>
          <Table.ColumnHeader>Medicine</Table.ColumnHeader>
          <Table.ColumnHeader>{t("analytics.margin.revenue")}</Table.ColumnHeader>
          <Table.ColumnHeader>{t("analytics.margin.cogs")}</Table.ColumnHeader>
          <Table.ColumnHeader>{t("analytics.margin.gross")}</Table.ColumnHeader>
          <Table.ColumnHeader>{t("analytics.margin.marginPct")}</Table.ColumnHeader>
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {rows.map((r) => (
          <Table.Row key={r.medicineId}>
            <Table.Cell>{r.sku}</Table.Cell>
            <Table.Cell>{r.medicineName}</Table.Cell>
            <Table.Cell>{formatMoney(Number(r.revenue))}</Table.Cell>
            <Table.Cell>{formatMoney(Number(r.cogs))}</Table.Cell>
            <Table.Cell>{formatMoney(Number(r.grossMargin))}</Table.Cell>
            <Table.Cell>{(r.marginPct * 100).toFixed(1)}%</Table.Cell>
          </Table.Row>
        ))}
        {rows.length === 0 && (
          <Table.Row>
            <Table.Cell colSpan={6}>
              <Text color="fg.muted" textAlign="center" py={4}>
                —
              </Text>
            </Table.Cell>
          </Table.Row>
        )}
      </Table.Body>
    </Table.Root>
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
