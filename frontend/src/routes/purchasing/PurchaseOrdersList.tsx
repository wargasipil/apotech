import {
  Badge,
  Box,
  Button,
  HStack,
  Input,
  Spinner,
  Stack,
  Switch,
  Table,
  Text,
} from "@chakra-ui/react";
import { Plus, Search } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import DateRangeFilter, { resolveRange, type DateRange } from "../../components/DateRangeFilter";
import EnumSelect from "../../components/EnumSelect";
import ExportButton from "../../components/ExportButton";
import Pagination from "../../components/Pagination";
import SearchableSelect from "../../components/SearchableSelect";
import {
  POStatus,
  type PurchaseOrder,
} from "../../gen/purchasing_iface/v1/order_pb";
import { downloadCsv } from "../../lib/csv";
import { formatMoney, formatDate } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { fetchPurchaseOrdersForExport, usePurchaseOrdersQuery } from "../../queries/purchasing";
import { searchSuppliers } from "../../queries/suppliers";
import { resolveSupplierMap, useSupplierRefs } from "../../queries/refs";

type Props = { status?: POStatus };

const STATUS_BADGE_PALETTE: Record<POStatus, string> = {
  [POStatus.PO_STATUS_UNSPECIFIED]: "gray",
  [POStatus.PO_STATUS_DRAFT]: "gray",
  [POStatus.PO_STATUS_SENT]: "blue",
  [POStatus.PO_STATUS_PARTIALLY_RECEIVED]: "orange",
  [POStatus.PO_STATUS_RECEIVED]: "green",
  [POStatus.PO_STATUS_CLOSED]: "green",
  [POStatus.PO_STATUS_VOIDED]: "red",
};

type DateField = "off" | "created" | "received";

export default function PurchaseOrdersList({ status = POStatus.PO_STATUS_UNSPECIFIED }: Props) {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [supplierFilter, setSupplierFilter] = useState("");
  const [onlyOutstanding, setOnlyOutstanding] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  const [dateField, setDateField] = useState<DateField>("off");
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));

  // Debounce the search box (250ms) into the query that drives the request.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const useRange = dateField !== "off";
  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${status}|${supplierFilter}|${onlyOutstanding}|${query}|${dateField}|${useRange ? range.fromUnix : 0}|${useRange ? range.toUnix : 0}`,
  );
  const posQ = usePurchaseOrdersQuery({
    status,
    supplierId: supplierFilter,
    onlyOutstanding,
    query,
    fromUnix: useRange ? BigInt(range.fromUnix) : 0n,
    toUnix: useRange ? BigInt(range.toUnix) : 0n,
    dateField: useRange ? dateField : "",
    limit: pageSize,
    offset: page * pageSize,
  });

  // Resolve supplier names for the current page's rows + the active filter
  // (resolve-by-IDs; not a full supplier-list preload).
  const supplierRefs = useSupplierRefs(
    useMemo(
      () => [supplierFilter, ...posQ.rows.map((po) => po.supplierId)],
      [supplierFilter, posQ.rows],
    ),
  );
  const supplierLabelOf = (id: string) => {
    const r = supplierRefs.get(id);
    return r ? `${r.code} · ${r.name}` : undefined;
  };

  const onExport = async () => {
    const rows = await fetchPurchaseOrdersForExport({
      status,
      supplierId: supplierFilter,
      onlyOutstanding,
      query,
      fromUnix: useRange ? BigInt(range.fromUnix) : 0n,
      toUnix: useRange ? BigInt(range.toUnix) : 0n,
      dateField: useRange ? dateField : "",
    });
    const sup = await resolveSupplierMap(rows.map((po) => po.supplierId));
    downloadCsv(
      `restock-${new Date().toISOString().slice(0, 10)}.csv`,
      rows.map((po) => {
        const r = sup.get(po.supplierId);
        return {
          poNo: po.poNo || po.id.slice(0, 8),
          supplier: r ? `${r.code} · ${r.name}` : "—",
          items: po.items
            .map(
              (it) =>
                `${it.medicineName || it.medicineId.slice(0, 8)} ×${fmtUnitQty(it.orderedQty, it.unitName, it.unitFactor)}`,
            )
            .join("; "),
          status: t(`purchasing.states.${statusKey(po.status)}`),
          created: formatDate(new Date(Number(po.createdAt) * 1000)),
          received: po.receivedAt > 0n ? formatDate(new Date(Number(po.receivedAt) * 1000)) : "",
          invoiceNo: po.invoiceNo || "",
          total: Number(po.orderedTotal),
          outstanding: Number(po.outstanding),
        };
      }),
      [
        { key: "poNo", header: t("purchasing.poNo") },
        { key: "supplier", header: t("purchasing.supplier") },
        { key: "items", header: t("purchasing.item") },
        { key: "status", header: t("purchasing.status") },
        { key: "created", header: t("purchasing.created") },
        { key: "received", header: t("purchasing.received") },
        { key: "invoiceNo", header: t("purchasing.invoiceNo") },
        { key: "total", header: t("purchasing.totalOrdered") },
        { key: "outstanding", header: t("purchasing.outstanding") },
      ],
    );
  };

  return (
    <Stack gap={4}>
      <HStack justify="space-between" wrap="wrap" gap={2}>
        <HStack gap={2} wrap="wrap">
          <Box position="relative">
            <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
              <Search size={14} />
            </Box>
            <Input
              size="sm"
              pl={7}
              width="260px"
              placeholder={t("purchasing.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <SearchableSelect
            size="sm"
            width="220px"
            value={supplierFilter}
            onChange={setSupplierFilter}
            loadOptions={searchSuppliers}
            itemToString={(s) => `${s.code} · ${s.name}`}
            itemToValue={(s) => s.id}
            selectedLabel={supplierLabelOf(supplierFilter)}
            placeholder={`${t("purchasing.supplier")} —`}
          />
          <EnumSelect
            size="sm"
            width="150px"
            value={dateField}
            onChange={(v) => setDateField(v as DateField)}
            items={[
              { value: "off", label: t("purchasing.dateOff") },
              { value: "created", label: t("purchasing.dateCreated") },
              { value: "received", label: t("purchasing.dateReceived") },
            ]}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
          />
          {useRange && <DateRangeFilter value={range} onChange={setRange} />}
          <Switch.Root checked={onlyOutstanding} onCheckedChange={(d) => setOnlyOutstanding(d.checked)}>
            <Switch.HiddenInput />
            <Switch.Control />
            <Switch.Label>{t("purchasing.outstandingToggle")}</Switch.Label>
          </Switch.Root>
        </HStack>
        <HStack gap={2}>
          <ExportButton onExport={onExport} />
          <Button colorPalette="blue" onClick={() => navigate("/purchasing/new")}>
            <Plus size={16} />
            {t("purchasing.newPo")}
          </Button>
        </HStack>
      </HStack>

      {posQ.isLoading ? (
        <Box p={6} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("purchasing.poNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.supplier")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.item")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.status")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.created")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.received")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.invoiceNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.totalOrdered")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.outstanding")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {posQ.rows.map((po: PurchaseOrder) => (
              <Table.Row
                key={po.id}
                onClick={() => navigate(`/purchasing/${po.id}`)}
                cursor="pointer"
                _hover={{ bg: "bg.muted" }}
              >
                <Table.Cell fontFamily="mono">{po.poNo || po.id.slice(0, 8)}</Table.Cell>
                <Table.Cell>{supplierLabelOf(po.supplierId) ?? "—"}</Table.Cell>
                <Table.Cell>
                  <ItemsList po={po} moreLabel={t("purchasing.itemsMore")} />
                </Table.Cell>
                <Table.Cell>
                  <Badge colorPalette={STATUS_BADGE_PALETTE[po.status]}>
                    {t(`purchasing.states.${statusKey(po.status)}`)}
                  </Badge>
                </Table.Cell>
                <Table.Cell>{formatDate(new Date(Number(po.createdAt) * 1000))}</Table.Cell>
                <Table.Cell>
                  {po.receivedAt > 0n ? formatDate(new Date(Number(po.receivedAt) * 1000)) : "—"}
                </Table.Cell>
                <Table.Cell fontFamily="mono">{po.invoiceNo || "—"}</Table.Cell>
                <Table.Cell fontFamily="mono">{formatMoney(Number(po.orderedTotal))}</Table.Cell>
                <Table.Cell fontFamily="mono">{formatMoney(Number(po.outstanding))}</Table.Cell>
              </Table.Row>
            ))}
            {posQ.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={9}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("common.noResults")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      )}

      <Pagination
        page={page}
        pageSize={pageSize}
        total={posQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />
    </Stack>
  );
}

// fmtUnitQty renders a BASE-unit quantity in its purchasable unit, e.g.
// (500, "box", 100n) -> "5 box". Falls back to the bare number when no unit.
function fmtUnitQty(qty: number, unitName: string, factor: bigint): string {
  const f = Number(factor) || 1;
  const q = f > 1 ? qty / f : qty;
  return unitName ? `${q} ${unitName}` : String(q);
}

function ItemsList({ po, moreLabel }: { po: PurchaseOrder; moreLabel: string }) {
  if (po.items.length === 0) return <Text color="fg.muted">—</Text>;
  const shown = po.items.slice(0, 3);
  const extra = po.items.length - shown.length;
  return (
    <Stack gap={0}>
      {shown.map((it) => (
        <Text key={it.id} fontSize="xs">
          <Text as="span" fontWeight="semibold">
            {it.medicineName || it.medicineId.slice(0, 8)}
            {it.medicineSku ? ` (${it.medicineSku})` : ""}
          </Text>
          {" ×" + fmtUnitQty(it.orderedQty, it.unitName, it.unitFactor)}
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

function statusKey(s: POStatus): string {
  switch (s) {
    case POStatus.PO_STATUS_DRAFT:
      return "draft";
    case POStatus.PO_STATUS_SENT:
      return "sent";
    case POStatus.PO_STATUS_PARTIALLY_RECEIVED:
      return "partiallyReceived";
    case POStatus.PO_STATUS_RECEIVED:
      return "received";
    case POStatus.PO_STATUS_CLOSED:
      return "closed";
    case POStatus.PO_STATUS_VOIDED:
      return "voided";
    default:
      return "draft";
  }
}
