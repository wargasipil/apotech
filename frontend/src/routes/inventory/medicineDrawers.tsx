import { Button, HStack, Stack, Switch } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import EntityDialog from "../../components/EntityDialog";
import FormField from "../../components/FormField";
import type { Medicine } from "../../gen/inventory_iface/v1/medicine_pb";
import { toast } from "../../lib/toaster";
import { useCreateMedicineMutation, useUpdateMedicineMutation } from "../../queries/medicines";

const Schema = z.object({
  sku: z.string().min(1),
  name: z.string().min(1),
  manufacturer: z.string(),
  unit: z.string().min(1),
  unitPrice: z.coerce.bigint().min(0n),
  prescriptionRequired: z.boolean(),
});
type FormValues = z.infer<typeof Schema>;

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
      <FormField control={form.control} name="name" label={t("inventory.medicines.name")} required />
      <FormField
        control={form.control}
        name="manufacturer"
        label={t("inventory.medicines.manufacturer")}
      />
      <FormField control={form.control} name="unit" label={t("inventory.medicines.unit")} required />
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

export function CreateMedicineDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
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
    <EntityDialog
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
    </EntityDialog>
  );
}

// Edit form is form-only — price history lives on the detail page now.
export function EditMedicineDialog({
  medicine,
  onClose,
}: {
  medicine: Medicine | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const update = useUpdateMedicineMutation();
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
    <EntityDialog
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
    </EntityDialog>
  );
}
