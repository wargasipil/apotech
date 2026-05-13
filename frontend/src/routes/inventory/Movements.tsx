import { useMemo, useState } from "react";
import {
  Box,
  Button,
  HStack,
  NativeSelect,
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
import FormField from "../../components/FormField";
import { MovementType } from "../../gen/inventory_iface/v1/stock_pb";
import { formatUnix } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useBatchesQuery } from "../../queries/batches";
import { useMedicinesQuery } from "../../queries/medicines";
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
  const movementsQ = useMovementsQuery({ batchId: filterBatch || undefined });
  const batchesQ = useBatchesQuery();
  const medicinesQ = useMedicinesQuery();

  const medById = useMemo(
    () => new Map((medicinesQ.data ?? []).map((m) => [m.id, m])),
    [medicinesQ.data],
  );
  const batchById = useMemo(
    () => new Map((batchesQ.data ?? []).map((b) => [b.id, b])),
    [batchesQ.data],
  );

  return (
    <Stack gap={4}>
      <HStack justify="space-between">
        <HStack gap={2}>
          <Text fontSize="sm" color="fg.muted">
            {t("inventory.movements.filterByBatch")}
          </Text>
          <NativeSelect.Root size="sm" width="auto">
            <NativeSelect.Field
              value={filterBatch}
              onChange={(e) => setFilterBatch(e.target.value)}
            >
              <option value="">{t("inventory.movements.filterAll")}</option>
              {batchesQ.data?.map((b) => (
                <option key={b.id} value={b.id}>
                  {medById.get(b.medicineId)?.name ?? b.medicineId} ·{" "}
                  {b.batchNumber || b.id.slice(0, 8)}
                </option>
              ))}
            </NativeSelect.Field>
            <NativeSelect.Indicator />
          </NativeSelect.Root>
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
            {movementsQ.data?.map((m) => {
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
            {(movementsQ.data?.length ?? 0) === 0 && (
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

      <RecordDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        batches={batchesQ.data ?? []}
        medById={medById}
      />
    </Stack>
  );
}

function RecordDrawer({
  open,
  onClose,
  batches,
  medById,
}: {
  open: boolean;
  onClose: () => void;
  batches: { id: string; medicineId: string; batchNumber: string; currentQuantity: bigint }[];
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
            <NativeSelect.Root>
              <NativeSelect.Field
                value={form.watch("batchId")}
                onChange={(e) => form.setValue("batchId", e.target.value)}
              >
                <option value="">{t("inventory.batches.selectMedicine")}</option>
                {batches.map((b) => (
                  <option key={b.id} value={b.id}>
                    {medById.get(b.medicineId)?.name ?? b.medicineId} ·{" "}
                    {b.batchNumber || b.id.slice(0, 8)} (qty {String(b.currentQuantity)})
                  </option>
                ))}
              </NativeSelect.Field>
              <NativeSelect.Indicator />
            </NativeSelect.Root>
          </Stack>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.movements.type")} *
            </Text>
            <NativeSelect.Root>
              <NativeSelect.Field
                value={String(form.watch("type"))}
                onChange={(e) => form.setValue("type", Number(e.target.value))}
              >
                <option value={MovementType.ADJUSTMENT}>
                  {t("inventory.movements.types.adjustment")}
                </option>
                <option value={MovementType.WRITE_OFF}>
                  {t("inventory.movements.types.writeOff")}
                </option>
              </NativeSelect.Field>
              <NativeSelect.Indicator />
            </NativeSelect.Root>
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
