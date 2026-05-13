import { useState } from "react";
import {
  Box,
  Button,
  HStack,
  Spinner,
  Stack,
  Switch,
  Table,
  Text,
} from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { Pencil, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import FormField from "../../components/FormField";
import { Medicine } from "../../gen/inventory_iface/v1/medicine_pb";
import { formatMoney, formatUnix } from "../../lib/format";
import { toast } from "../../lib/toaster";
import {
  useCreateMedicineMutation,
  useMedicinePricesQuery,
  useMedicinesQuery,
  useUpdateMedicineMutation,
} from "../../queries/medicines";

const Schema = z.object({
  sku: z.string().min(1),
  name: z.string().min(1),
  manufacturer: z.string(),
  unit: z.string().min(1),
  unitPrice: z.coerce.bigint().min(0n),
  prescriptionRequired: z.boolean(),
});
type FormValues = z.infer<typeof Schema>;

export default function Medicines() {
  const { t } = useTranslation();
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Medicine | null>(null);
  const medicinesQ = useMedicinesQuery();

  return (
    <Stack gap={4}>
      <HStack justify="flex-end">
        <Button size="sm" colorPalette="blue" onClick={() => setCreateOpen(true)}>
          <Plus size={16} />
          {t("inventory.medicines.addTitle")}
        </Button>
      </HStack>

      {medicinesQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("inventory.medicines.sku")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.medicines.name")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.medicines.unit")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.medicines.unitPrice")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.medicines.rxShort")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {medicinesQ.data?.map((m) => (
              <Table.Row key={m.id}>
                <Table.Cell>{m.sku}</Table.Cell>
                <Table.Cell>{m.name}</Table.Cell>
                <Table.Cell>{m.unit}</Table.Cell>
                <Table.Cell>{formatMoney(m.unitPrice)}</Table.Cell>
                <Table.Cell>{m.prescriptionRequired ? t("common.yes") : t("common.no")}</Table.Cell>
                <Table.Cell>
                  <Button size="xs" variant="ghost" onClick={() => setEditing(m)}>
                    <Pencil size={14} />
                    {t("common.edit")}
                  </Button>
                </Table.Cell>
              </Table.Row>
            ))}
            {(medicinesQ.data?.length ?? 0) === 0 && (
              <Table.Row>
                <Table.Cell colSpan={6}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("common.noResults")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      )}

      <CreateDrawer open={createOpen} onClose={() => setCreateOpen(false)} />
      <EditDrawer medicine={editing} onClose={() => setEditing(null)} />
    </Stack>
  );
}

function MedicineForm({
  form,
  isCreate = true,
}: {
  form: ReturnType<typeof useForm<FormValues>>;
  isCreate?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Stack gap={4}>
      <FormField
        control={form.control}
        name="sku"
        label={t("inventory.medicines.sku")}
        required={isCreate}
        autoFocus={isCreate}
      />
      <FormField
        control={form.control}
        name="name"
        label={t("inventory.medicines.name")}
        required
      />
      <FormField
        control={form.control}
        name="manufacturer"
        label={t("inventory.medicines.manufacturer")}
      />
      <FormField
        control={form.control}
        name="unit"
        label={t("inventory.medicines.unit")}
        required
      />
      <FormField
        control={form.control}
        name="unitPrice"
        label={t("inventory.medicines.unitPrice")}
        type="number"
        inputMode="numeric"
        required
      />
      <Switch.Root
        checked={form.watch("prescriptionRequired")}
        onCheckedChange={(d) => form.setValue("prescriptionRequired", d.checked)}
      >
        <Switch.HiddenInput />
        <Switch.Control />
        <Switch.Label>{t("inventory.medicines.rx")}</Switch.Label>
      </Switch.Root>
    </Stack>
  );
}

function CreateDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateMedicineMutation();
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: {
      sku: "",
      name: "",
      manufacturer: "",
      unit: "tablet",
      unitPrice: 0n,
      prescriptionRequired: false,
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
      title={t("inventory.medicines.addTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending}>
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <MedicineForm form={form} isCreate />
      </form>
    </EntityDrawer>
  );
}

function EditDrawer({ medicine, onClose }: { medicine: Medicine | null; onClose: () => void }) {
  const { t } = useTranslation();
  const update = useUpdateMedicineMutation();
  const pricesQ = useMedicinePricesQuery(medicine?.id ?? "", !!medicine);
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    values: medicine
      ? {
          sku: medicine.sku,
          name: medicine.name,
          manufacturer: medicine.manufacturer,
          unit: medicine.unit,
          unitPrice: medicine.unitPrice,
          prescriptionRequired: medicine.prescriptionRequired,
        }
      : undefined,
  });

  const submit = form.handleSubmit(async (values) => {
    if (!medicine) return;
    try {
      await update.mutateAsync({
        id: medicine.id,
        name: values.name,
        manufacturer: values.manufacturer,
        unit: values.unit,
        unitPrice: values.unitPrice,
        prescriptionRequired: values.prescriptionRequired,
      });
      toast.success(t("common.save") + " ✓");
      onClose();
    } catch {
      /* toast handled globally */
    }
  });

  return (
    <EntityDrawer
      open={!!medicine}
      onClose={onClose}
      title={medicine ? `${t("inventory.medicines.editTitle")} · ${medicine.sku}` : ""}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={update.isPending}>
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <MedicineForm form={form} />
      </form>
      <Box mt={6}>
        <Text fontSize="sm" fontWeight="semibold" mb={2}>
          {t("inventory.medicines.priceHistory")}
        </Text>
        {!pricesQ.data || pricesQ.data.length === 0 ? (
          <Text fontSize="sm" color="fg.muted">
            {t("inventory.medicines.priceHistoryEmpty")}
          </Text>
        ) : (
          <Table.Root size="sm">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>{t("inventory.medicines.priceFrom")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.medicines.priceTo")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.medicines.pricePrice")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {pricesQ.data.map((p) => (
                <Table.Row key={p.id}>
                  <Table.Cell>{formatUnix(p.effectiveFrom)}</Table.Cell>
                  <Table.Cell>
                    {p.effectiveTo > 0n
                      ? formatUnix(p.effectiveTo)
                      : t("inventory.medicines.priceCurrent")}
                  </Table.Cell>
                  <Table.Cell>{formatMoney(p.unitPrice)}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        )}
      </Box>
    </EntityDrawer>
  );
}

