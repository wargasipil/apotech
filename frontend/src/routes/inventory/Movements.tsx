import { useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  HStack,
  Input,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { Plus, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import DateRangeFilter, { resolveRange, type DateRange } from "../../components/DateRangeFilter";
import EntityDrawer from "../../components/EntityDrawer";
import EnumSelect from "../../components/EnumSelect";
import FormField from "../../components/FormField";
import ExportButton from "../../components/ExportButton";
import Pagination from "../../components/Pagination";
import SearchableSelect from "../../components/SearchableSelect";
import { searchBatches } from "../../queries/batches";
import { MovementType } from "../../gen/inventory_iface/v1/stock_pb";
import { downloadCsv } from "../../lib/csv";
import { formatUnix } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import { resolveBatchMap, useBatchRefs } from "../../queries/refs";
import { fetchMovementsForExport, useMovementsQuery, useRecordMovementMutation } from "../../queries/stock";

const Schema = z.object({
  batchId: z.string().min(1),
  qty: z.coerce.number().int().refine((n) => n !== 0, "qty must not be zero"),
  type: z.coerce.number().int(),
  reason: z.string(),
});
type FormValues = z.infer<typeof Schema>;

function typeKey(type: MovementType): string {
  switch (type) {
    case MovementType.PURCHASE:
      return "purchase";
    case MovementType.SALE:
      return "sale";
    case MovementType.ADJUSTMENT:
      return "adjustment";
    case MovementType.WRITE_OFF:
      return "writeOff";
    default:
      return "unspecified";
  }
}

export default function Movements() {
  const { t } = useTranslation();
  const [filterBatch, setFilterBatch] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);
  const [dateOn, setDateOn] = useState(false);
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${filterBatch}|${query}|${dateOn ? range.fromUnix : 0}|${dateOn ? range.toUnix : 0}`,
  );
  const movementsQ = useMovementsQuery({
    batchId: filterBatch || undefined,
    query,
    fromUnix: dateOn ? range.fromUnix : 0,
    toUnix: dateOn ? range.toUnix : 0,
    page,
    pageSize,
  });
  // Resolve the page's batch refs (batch_number + medicine_name) + the active
  // filter batch (resolve-by-IDs; the batch filter still searches server-side).
  const batchRefs = useBatchRefs(
    useMemo(
      () => [filterBatch, ...movementsQ.rows.map((m) => m.batchId)],
      [filterBatch, movementsQ.rows],
    ),
  );

  const onExport = async () => {
    const rows = await fetchMovementsForExport({
      batchId: filterBatch || undefined,
      query,
      fromUnix: dateOn ? range.fromUnix : 0,
      toUnix: dateOn ? range.toUnix : 0,
    });
    const refs = await resolveBatchMap(rows.map((m) => m.batchId));
    downloadCsv(
      `movements-${new Date().toISOString().slice(0, 10)}.csv`,
      rows.map((m) => {
        const r = refs.get(m.batchId);
        return {
          date: formatUnix(m.createdAt),
          medicine: r?.medicineName ?? "—",
          batch: r?.batchNumber || "—",
          type: t(`inventory.movements.types.${typeKey(m.type)}`),
          qty: m.qty,
          reason: m.reason,
        };
      }),
      [
        { key: "date", header: t("inventory.movements.when") },
        { key: "medicine", header: t("inventory.batches.medicine") },
        { key: "batch", header: t("inventory.movements.batch") },
        { key: "type", header: t("inventory.movements.type") },
        { key: "qty", header: t("inventory.movements.qty") },
        { key: "reason", header: t("inventory.movements.reason") },
      ],
    );
  };

  return (
    <Stack gap={4}>
      <HStack justify="space-between" wrap="wrap" gap={2}>
        <HStack gap={2} wrap="wrap">
          <SearchableSelect
            size="sm"
            width="260px"
            value={filterBatch}
            onChange={setFilterBatch}
            loadOptions={(q) => searchBatches(q)}
            itemToString={(b) => b.batchNumber || b.id.slice(0, 8)}
            itemToValue={(b) => b.id}
            selectedLabel={(() => {
              const r = batchRefs.get(filterBatch);
              return r ? `${r.medicineName} · ${r.batchNumber || "—"}` : undefined;
            })()}
            placeholder={t("inventory.movements.filterAll")}
          />
          <Box position="relative">
            <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
              <Search size={14} />
            </Box>
            <Input
              size="sm"
              pl={7}
              width="220px"
              placeholder={t("inventory.movements.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <EnumSelect
            size="sm"
            width="150px"
            value={dateOn ? "on" : "off"}
            onChange={(v) => setDateOn(v === "on")}
            items={[
              { value: "off", label: t("common.anyDate") },
              { value: "on", label: t("common.dateRange") },
            ]}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
          />
          {dateOn && <DateRangeFilter value={range} onChange={setRange} />}
        </HStack>
        <HStack gap={2}>
          <ExportButton onExport={onExport} />
          <Button size="sm" colorPalette="blue" onClick={() => setDrawerOpen(true)}>
            <Plus size={16} />
            {t("inventory.movements.record")}
          </Button>
        </HStack>
      </HStack>

      {movementsQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("inventory.movements.when")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.movements.batch")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.movements.type")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.movements.qty")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.movements.reason")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {movementsQ.rows.map((m) => {
              const ref = batchRefs.get(m.batchId);
              return (
                <Table.Row key={m.id}>
                  <Table.Cell>{formatUnix(m.createdAt)}</Table.Cell>
                  <Table.Cell>
                    {ref?.medicineName ?? "—"} · {ref?.batchNumber || "—"}
                  </Table.Cell>
                  <Table.Cell>{t(`inventory.movements.types.${typeKey(m.type)}`)}</Table.Cell>
                  <Table.Cell>{m.qty > 0 ? `+${m.qty}` : m.qty}</Table.Cell>
                  <Table.Cell>{m.reason}</Table.Cell>
                </Table.Row>
              );
            })}
            {movementsQ.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={5}>
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
        total={movementsQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <RecordDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      />
    </Stack>
  );
}

function RecordDrawer({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const record = useRecordMovementMutation();
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: {
      batchId: "",
      qty: 0,
      type: MovementType.ADJUSTMENT,
      reason: "",
    },
  });

  const submit = form.handleSubmit(async (values) => {
    try {
      await record.mutateAsync({
        batchId: values.batchId,
        qty: values.qty,
        type: values.type as MovementType,
        reason: values.reason,
      });
      toast.success(t("common.create") + " ✓");
      form.reset();
      onClose();
    } catch {
      /* toast handled globally */
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("inventory.movements.recordTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={record.isPending}>
            {t("inventory.movements.record")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <Stack gap={4}>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.movements.batch")} *
            </Text>
            <SearchableSelect
              value={form.watch("batchId")}
              onChange={(v) => form.setValue("batchId", v)}
              loadOptions={(q) => searchBatches(q)}
              itemToString={(b) =>
                `${b.batchNumber || b.id.slice(0, 8)} (qty ${String(b.currentQuantity)})`
              }
              itemToValue={(b) => b.id}
              placeholder={t("inventory.batches.selectMedicine")}
            />
          </Stack>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.movements.type")} *
            </Text>
            <EnumSelect
              value={String(form.watch("type"))}
              onChange={(v) => form.setValue("type", Number(v))}
              items={[
                { value: String(MovementType.ADJUSTMENT), label: t("inventory.movements.types.adjustment") },
                { value: String(MovementType.WRITE_OFF), label: t("inventory.movements.types.writeOff") },
              ]}
              itemToString={(o) => o.label}
              itemToValue={(o) => o.value}
            />
          </Stack>
          <FormField
            control={form.control}
            name="qty"
            label={t("inventory.movements.qty")}
            type="number"
            required
          />
          <FormField
            control={form.control}
            name="reason"
            label={t("inventory.movements.reason")}
          />
        </Stack>
      </form>
    </EntityDrawer>
  );
}
