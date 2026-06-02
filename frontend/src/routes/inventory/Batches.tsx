import { useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  HStack,
  Input,
  Link as ChakraLink,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Link as RouterLink } from "react-router-dom";
import { zodResolver } from "@hookform/resolvers/zod";
import { Plus, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import DateRangeFilter, { resolveRange, type DateRange } from "../../components/DateRangeFilter";
import EnumSelect from "../../components/EnumSelect";
import EntityDrawer from "../../components/EntityDrawer";
import ExpiryBadge from "../../components/ExpiryBadge";
import FormField from "../../components/FormField";
import Pagination from "../../components/Pagination";
import SearchableSelect from "../../components/SearchableSelect";
import { searchMedicines } from "../../queries/medicines";
import { searchSuppliers } from "../../queries/suppliers";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import { useBatchesQuery, useCreateBatchMutation } from "../../queries/batches";
import { useMedicineRefs, useSupplierRefs } from "../../queries/refs";

const Schema = z.object({
  medicineId: z.string().min(1),
  supplierId: z.string(),
  batchNumber: z.string(),
  expiryDate: z.string().min(1),
  costPrice: z.coerce.bigint().min(0n),
  receivedAt: z.string(),
  initialQuantity: z.coerce.bigint().min(0n),
});
type FormValues = z.infer<typeof Schema>;

export default function Batches() {
  const { t } = useTranslation();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);
  const [dateField, setDateField] = useState<"off" | "received" | "expiry">("off");
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const useRange = dateField !== "off";
  const [supplierId, setSupplierId] = useState("");
  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${query}|${dateField}|${useRange ? range.fromUnix : 0}|${useRange ? range.toUnix : 0}|${supplierId}`,
  );
  const batchesQ = useBatchesQuery({
    // Scope rows to the active warehouse: only lots with stock here (backend
    // HAVING SUM(qty in warehouse) > 0). Qty is already active-warehouse.
    onlyInStock: true,
    query,
    supplierId,
    dateField: useRange ? dateField : "",
    fromUnix: useRange ? range.fromUnix : 0,
    toUnix: useRange ? range.toUnix : 0,
    page,
    pageSize,
  });
  // Resolve the page's medicine names (resolve-by-IDs; the CreateDrawer selects
  // use server-side search via loadOptions).
  const medRefs = useMedicineRefs(
    useMemo(() => batchesQ.rows.map((b) => b.medicineId), [batchesQ.rows]),
  );
  const supplierRefs = useSupplierRefs(
    useMemo(() => {
      const ids = new Set<string>();
      batchesQ.rows.forEach((b) => {
        if (b.supplierId) ids.add(b.supplierId);
      });
      // Include the active filter ID so the SearchableSelect's selectedLabel
      // resolves correctly on first render after navigation.
      if (supplierId) ids.add(supplierId);
      return Array.from(ids);
    }, [batchesQ.rows, supplierId]),
  );

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
              width="240px"
              placeholder={t("inventory.batches.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <EnumSelect
            size="sm"
            width="150px"
            value={dateField}
            onChange={(v) => setDateField(v as "off" | "received" | "expiry")}
            items={[
              { value: "off", label: t("common.anyDate") },
              { value: "received", label: t("inventory.batches.byReceived") },
              { value: "expiry", label: t("inventory.batches.byExpiry") },
            ]}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
          />
          {useRange && <DateRangeFilter value={range} onChange={setRange} />}
          <Box width="200px">
            <SearchableSelect
              size="sm"
              value={supplierId}
              onChange={setSupplierId}
              loadOptions={searchSuppliers}
              itemToString={(s) => `${s.code} · ${s.name}`}
              itemToValue={(s) => s.id}
              placeholder={t("inventory.batches.supplier")}
              selectedLabel={
                supplierId
                  ? (() => {
                      const s = supplierRefs.get(supplierId);
                      return s ? `${s.code} · ${s.name}` : undefined;
                    })()
                  : undefined
              }
            />
          </Box>
        </HStack>
        <Button size="sm" colorPalette="blue" onClick={() => setDrawerOpen(true)}>
          <Plus size={16} />
          {t("inventory.batches.addTitle")}
        </Button>
      </HStack>

      {batchesQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("inventory.batches.medicine")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.batches.batchNumber")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.batches.supplier")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.batches.expiry")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.batches.cost")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.batches.qty")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.batches.po")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {batchesQ.rows.map((b) => (
              <Table.Row key={b.id}>
                <Table.Cell>{medRefs.get(b.medicineId)?.name ?? "—"}</Table.Cell>
                <Table.Cell>{b.batchNumber || "—"}</Table.Cell>
                <Table.Cell>
                  {b.supplierId
                    ? (() => {
                        const s = supplierRefs.get(b.supplierId);
                        return s ? `${s.code} · ${s.name}` : "—";
                      })()
                    : "—"}
                </Table.Cell>
                <Table.Cell>
                  <HStack gap={2}>
                    <Text>{b.expiryDate}</Text>
                    <ExpiryBadge expiry={b.expiryDate} />
                  </HStack>
                </Table.Cell>
                <Table.Cell>{formatMoney(b.costPrice)}</Table.Cell>
                <Table.Cell>{String(b.currentQuantity)}</Table.Cell>
                <Table.Cell fontFamily="mono">
                  {b.purchaseOrderId ? (
                    <ChakraLink asChild colorPalette="blue">
                      <RouterLink to={`/purchasing/${b.purchaseOrderId}`}>
                        {b.poNo || b.purchaseOrderId.slice(0, 8)}
                      </RouterLink>
                    </ChakraLink>
                  ) : (
                    <Text color="fg.muted">—</Text>
                  )}
                </Table.Cell>
              </Table.Row>
            ))}
            {batchesQ.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={7}>
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
        total={batchesQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <CreateDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </Stack>
  );
}

function CreateDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateBatchMutation();
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: {
      medicineId: "",
      supplierId: "",
      batchNumber: "",
      expiryDate: "",
      costPrice: 0n,
      receivedAt: "",
      initialQuantity: 0n,
    },
  });

  const submit = form.handleSubmit(async (values) => {
    try {
      await create.mutateAsync(values);
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
      title={t("inventory.batches.addTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending}>
            {t("inventory.batches.receive")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <Stack gap={4}>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.batches.medicine")} *
            </Text>
            <SearchableSelect
              value={form.watch("medicineId")}
              onChange={(v) => form.setValue("medicineId", v)}
              loadOptions={searchMedicines}
              itemToString={(m) => `${m.sku} · ${m.name}`}
              itemToValue={(m) => m.id}
              placeholder={t("inventory.batches.selectMedicine")}
            />
          </Stack>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.batches.supplier")}
            </Text>
            <SearchableSelect
              value={form.watch("supplierId")}
              onChange={(v) => form.setValue("supplierId", v)}
              loadOptions={searchSuppliers}
              itemToString={(s) => `${s.code} · ${s.name}`}
              itemToValue={(s) => s.id}
              placeholder={t("inventory.batches.supplierNone")}
            />
          </Stack>
          <FormField
            control={form.control}
            name="batchNumber"
            label={t("inventory.batches.batchNumber")}
          />
          <FormField
            control={form.control}
            name="expiryDate"
            label={t("inventory.batches.expiry")}
            type="date"
            required
          />
          <FormField
            control={form.control}
            name="receivedAt"
            label={t("inventory.batches.received")}
            type="date"
          />
          <FormField
            control={form.control}
            name="costPrice"
            label={t("inventory.batches.costPerUnit")}
            money
          />
          <FormField
            control={form.control}
            name="initialQuantity"
            label={t("inventory.batches.initialQty")}
            type="number"
            inputMode="numeric"
            required
          />
        </Stack>
      </form>
    </EntityDrawer>
  );
}
