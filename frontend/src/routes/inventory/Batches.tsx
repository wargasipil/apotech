import { useMemo, useState } from "react";
import {
  Badge,
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
import FormField from "../../components/FormField";
import SearchableSelect from "../../components/SearchableSelect";
import { searchMedicines } from "../../queries/medicines";
import { searchSuppliers } from "../../queries/suppliers";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useBatchesQuery, useCreateBatchMutation } from "../../queries/batches";
import { useMedicinesQuery } from "../../queries/medicines";

const MS_PER_DAY = 86_400_000;

function ExpiryBadge({ expiry, expiredLabel }: { expiry: string; expiredLabel: string }) {
  const days = Math.ceil((new Date(expiry).getTime() - Date.now()) / MS_PER_DAY);
  if (days <= 0) return <Badge colorPalette="red">{expiredLabel}</Badge>;
  if (days <= 30) return <Badge colorPalette="red">{days}d</Badge>;
  if (days <= 90) return <Badge colorPalette="orange">{days}d</Badge>;
  return <Badge colorPalette="green">{days}d</Badge>;
}

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
  const batchesQ = useBatchesQuery();
  // Kept for the medicine-name column display on the batches table; the
  // selects inside CreateDrawer use server-side search via loadOptions.
  const medicinesQ = useMedicinesQuery();

  const medById = useMemo(
    () => new Map((medicinesQ.data ?? []).map((m) => [m.id, m])),
    [medicinesQ.data],
  );

  return (
    <Stack gap={4}>
      <HStack justify="flex-end">
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
              <Table.ColumnHeader>{t("inventory.batches.expiry")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.batches.cost")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.batches.qty")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {batchesQ.data?.map((b) => (
              <Table.Row key={b.id}>
                <Table.Cell>{medById.get(b.medicineId)?.name ?? b.medicineId}</Table.Cell>
                <Table.Cell>{b.batchNumber || "—"}</Table.Cell>
                <Table.Cell>
                  <HStack gap={2}>
                    <Text>{b.expiryDate}</Text>
                    <ExpiryBadge
                      expiry={b.expiryDate}
                      expiredLabel={t("inventory.batches.expired")}
                    />
                  </HStack>
                </Table.Cell>
                <Table.Cell>{formatMoney(b.costPrice)}</Table.Cell>
                <Table.Cell>{String(b.currentQuantity)}</Table.Cell>
              </Table.Row>
            ))}
            {(batchesQ.data?.length ?? 0) === 0 && (
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
              itemToString={(s) => s.name}
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
            type="number"
            inputMode="numeric"
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
