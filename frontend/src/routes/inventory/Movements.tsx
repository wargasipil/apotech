import { useMemo, useState } from "react";
import {
  Box,
  Button,
  HStack,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import EnumSelect from "../../components/EnumSelect";
import FormField from "../../components/FormField";
import Pagination from "../../components/Pagination";
import SearchableSelect from "../../components/SearchableSelect";
import { searchBatches } from "../../queries/batches";
import { MovementType } from "../../gen/inventory_iface/v1/stock_pb";
import { formatUnix } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import { useAllBatchesQuery } from "../../queries/batches";
import { useAllMedicinesQuery } from "../../queries/medicines";
import { useMovementsQuery, useRecordMovementMutation } from "../../queries/stock";

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
  const { page, setPage, pageSize, setPageSize } = usePageState(filterBatch);
  const movementsQ = useMovementsQuery({ batchId: filterBatch || undefined, page, pageSize });
  const batchesQ = useAllBatchesQuery();
  const medicinesQ = useAllMedicinesQuery();

  const medById = useMemo(
    () => new Map(medicinesQ.rows.map((m) => [m.id, m])),
    [medicinesQ.rows],
  );
  const batchById = useMemo(
    () => new Map(batchesQ.rows.map((b) => [b.id, b])),
    [batchesQ.rows],
  );

  return (
    <Stack gap={4}>
      <HStack justify="space-between">
        <HStack gap={2}>
          <Text fontSize="sm" color="fg.muted">
            {t("inventory.movements.filterByBatch")}
          </Text>
          <SearchableSelect
            size="sm"
            width="280px"
            value={filterBatch}
            onChange={setFilterBatch}
            loadOptions={(q) => searchBatches(q)}
            itemToString={(b) =>
              `${medById.get(b.medicineId)?.name ?? b.medicineId} · ${b.batchNumber || b.id.slice(0, 8)}`
            }
            itemToValue={(b) => b.id}
            selectedLabel={(() => {
              const b = batchById.get(filterBatch);
              return b
                ? `${medById.get(b.medicineId)?.name ?? b.medicineId} · ${b.batchNumber || b.id.slice(0, 8)}`
                : undefined;
            })()}
            placeholder={t("inventory.movements.filterAll")}
          />
        </HStack>
        <Button size="sm" colorPalette="blue" onClick={() => setDrawerOpen(true)}>
          <Plus size={16} />
          {t("inventory.movements.record")}
        </Button>
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
              const batch = batchById.get(m.batchId);
              const medName = batch ? medById.get(batch.medicineId)?.name : undefined;
              return (
                <Table.Row key={m.id}>
                  <Table.Cell>{formatUnix(m.createdAt)}</Table.Cell>
                  <Table.Cell>
                    {medName ?? "?"} · {batch?.batchNumber || m.batchId.slice(0, 8)}
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
        medById={medById}
      />
    </Stack>
  );
}

function RecordDrawer({
  open,
  onClose,
  medById,
}: {
  open: boolean;
  onClose: () => void;
  medById: Map<string, { name: string }>;
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
                `${medById.get(b.medicineId)?.name ?? b.medicineId} · ${b.batchNumber || b.id.slice(0, 8)} (qty ${String(b.currentQuantity)})`
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
