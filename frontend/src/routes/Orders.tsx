import {
  Badge,
  Box,
  HStack,
  Input,
  Spinner,
  Stack,
  Table,
  Tabs,
  Text,
} from "@chakra-ui/react";
import { Search } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import DateRangeFilter, { resolveRange, type DateRange } from "../components/DateRangeFilter";
import ExportButton from "../components/ExportButton";
import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import { SaleStatus, type SaleItem } from "../gen/pos_iface/v1/sale_pb";
import { downloadCsv } from "../lib/csv";
import { formatMoney, formatUnix } from "../lib/format";
import { usePageState } from "../lib/pagination";
import { resolveUserMap, useUserRefs } from "../queries/refs";
import { fetchSalesForExport, useListSalesQuery, useSalesSummaryQuery } from "../queries/sales";

const PAYMENT_KEY: Record<number, string> = {
  0: "unspecified",
  1: "cash",
  2: "bpjs",
  3: "insuranceOther",
};

const STATUS_BADGE: Record<number, string> = {
  [SaleStatus.UNSPECIFIED]: "gray",
  [SaleStatus.DRAFT]: "gray",
  [SaleStatus.COMPLETED]: "green",
  [SaleStatus.VOIDED]: "red",
};

function statusKey(s: SaleStatus): string {
  switch (s) {
    case SaleStatus.DRAFT:
      return "draft";
    case SaleStatus.COMPLETED:
      return "completed";
    case SaleStatus.VOIDED:
      return "voided";
    default:
      return "unspecified";
  }
}

export default function Orders() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>(String(SaleStatus.COMPLETED));
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));

  // Debounce the search box (250ms) into the query that drives the request.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const status = Number(statusFilter) as SaleStatus;
  const fromUnix = BigInt(range.fromUnix);
  const toUnix = BigInt(range.toUnix);

  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${query}|${status}|${range.fromUnix}|${range.toUnix}`,
  );

  // List is server-paginated (one page of rows). The summary is a SEPARATE
  // server-side aggregate over ALL matching rows — same filters, no page bound.
  const salesQ = useListSalesQuery({
    query,
    status,
    fromUnix,
    toUnix,
    limit: pageSize,
    offset: page * pageSize,
  });
  const summaryQ = useSalesSummaryQuery({ query, status, fromUnix, toUnix });
  const summary = summaryQ.data;

  // Resolve cashier names for the "Created by" column. Sale rows already carry
  // cashier_user_id; the metric RPC pattern keeps names off the wire and the
  // frontend looks them up via the shared Resolve<Domain>(ids) hook.
  const cashierIds = useMemo(
    () => salesQ.rows.map((s) => s.cashierUserId).filter(Boolean),
    [salesQ.rows],
  );
  const userRefs = useUserRefs(cashierIds);
  const createdByLabel = (cashierId: string) =>
    userRefs.get(cashierId)?.name || userRefs.get(cashierId)?.email || "—";

  const onExport = async () => {
    const rows = await fetchSalesForExport({ query, status, fromUnix, toUnix });
    // Bulk-resolve cashier names for the export's rows (mirrors how the
    // page table resolves them, but imperative since this isn't a hook).
    const users = await resolveUserMap(rows.map((s) => s.cashierUserId).filter(Boolean));
    downloadCsv(
      `order-history-${new Date().toISOString().slice(0, 10)}.csv`,
      rows.map((s) => ({
        saleNo: s.saleNo || s.id.slice(0, 8),
        date: formatUnix(s.createdAt),
        createdBy: users.get(s.cashierUserId)?.name || users.get(s.cashierUserId)?.email || "",
        customer: s.customerName || "",
        items: s.items
          .map(
            (it) =>
              `${it.medicineName || it.medicineId.slice(0, 8)} ×${it.qty}${it.unitName ? " " + it.unitName : ""}`,
          )
          .join("; "),
        payment: t(`orders.payments.${PAYMENT_KEY[s.paymentSource] ?? "unspecified"}`),
        total: Number(s.total),
        status: t(`orders.states.${statusKey(s.status)}`),
      })),
      [
        { key: "saleNo", header: t("orders.saleNo") },
        { key: "date", header: t("orders.date") },
        { key: "createdBy", header: t("orders.createdBy") },
        { key: "customer", header: t("orders.customer") },
        { key: "items", header: t("orders.items") },
        { key: "payment", header: t("orders.payment") },
        { key: "total", header: t("orders.total") },
        { key: "status", header: t("orders.status") },
      ],
    );
  };

  return (
    <Box>
      <PageHeader breadcrumbs={[{ label: t("orders.title") }]} title={t("orders.title")} />

      {/* Status filter as tabs (state-driven; one route). */}
      <Tabs.Root
        value={statusFilter}
        onValueChange={(d) => setStatusFilter(d.value)}
        variant="line"
        mb={4}
      >
        <Tabs.List>
          <Tabs.Trigger value={String(SaleStatus.UNSPECIFIED)}>{t("orders.statusAll")}</Tabs.Trigger>
          <Tabs.Trigger value={String(SaleStatus.COMPLETED)}>{t("orders.states.completed")}</Tabs.Trigger>
          <Tabs.Trigger value={String(SaleStatus.VOIDED)}>{t("orders.states.voided")}</Tabs.Trigger>
        </Tabs.List>
      </Tabs.Root>

      {/* Search + always-visible date range. */}
      <HStack mb={4} gap={3} wrap="wrap">
        <Box position="relative">
          <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
            <Search size={14} />
          </Box>
          <Input
            size="sm"
            pl={7}
            width="280px"
            placeholder={t("orders.searchPlaceholder")}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
          />
        </Box>
        <DateRangeFilter value={range} onChange={setRange} />
        <ExportButton onExport={onExport} />
      </HStack>

      {/* Summary over all matching orders (server-side aggregate). */}
      <HStack mb={4} gap={3} wrap="wrap">
        <SummaryTile
          label={t("orders.summary.sales")}
          value={String(summary?.saleCount ?? 0n)}
        />
        <SummaryTile
          label={t("orders.summary.itemsSold")}
          value={String(summary?.itemsSold ?? 0n)}
        />
        <SummaryTile
          label={t("orders.summary.revenue")}
          value={formatMoney(summary?.revenue ?? 0n)}
        />
      </HStack>

      {salesQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("orders.saleNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("orders.date")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("orders.createdBy")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("orders.customer")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("orders.items")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("orders.payment")}</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">{t("orders.total")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("orders.status")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {salesQ.rows.map((s) => (
              <Table.Row
                key={s.id}
                cursor="pointer"
                _hover={{ bg: "bg.muted" }}
                onClick={() => navigate(`/orders/${s.id}`)}
              >
                <Table.Cell fontFamily="mono">{s.saleNo || s.id.slice(0, 8)}</Table.Cell>
                <Table.Cell>{formatUnix(s.createdAt)}</Table.Cell>
                <Table.Cell>{createdByLabel(s.cashierUserId)}</Table.Cell>
                <Table.Cell>{s.customerName || "—"}</Table.Cell>
                <Table.Cell>
                  <ItemsSummary items={s.items} moreLabel={t("orders.itemsMore")} />
                </Table.Cell>
                <Table.Cell>{t(`orders.payments.${PAYMENT_KEY[s.paymentSource] ?? "unspecified"}`)}</Table.Cell>
                <Table.Cell textAlign="end" fontFamily="mono">{formatMoney(s.total)}</Table.Cell>
                <Table.Cell>
                  <Badge colorPalette={STATUS_BADGE[s.status] ?? "gray"}>
                    {t(`orders.states.${statusKey(s.status)}`)}
                  </Badge>
                </Table.Cell>
              </Table.Row>
            ))}
            {salesQ.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={8}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("common.noResults")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      )}

      <Box mt={3}>
        <Pagination
          page={page}
          pageSize={pageSize}
          total={salesQ.total}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
        />
      </Box>
    </Box>
  );
}

function SummaryTile({ label, value }: { label: string; value: string }) {
  return (
    <Box flex="1" minW="160px" bg="bg.subtle" borderWidth="1px" borderRadius="lg" px={4} py={3}>
      <Text fontSize="xs" color="fg.muted">
        {label}
      </Text>
      <Text fontSize="xl" fontWeight="semibold">
        {value}
      </Text>
    </Box>
  );
}

function ItemsSummary({ items, moreLabel }: { items: SaleItem[]; moreLabel: string }) {
  if (items.length === 0) return <Text color="fg.muted">—</Text>;
  const shown = items.slice(0, 3);
  const extra = items.length - shown.length;
  return (
    <Stack gap={0}>
      {shown.map((it) => (
        <Text key={it.id} fontSize="xs">
          {(it.medicineName || it.medicineId.slice(0, 8)) +
            " ×" +
            it.qty +
            (it.unitName ? " " + it.unitName : "")}
        </Text>
      ))}
      {extra > 0 && (
        <Text fontSize="xs" color="fg.muted">
          {moreLabel.replace("{{count}}", String(extra))}
        </Text>
      )}
    </Stack>
  );
}
