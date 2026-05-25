import { Button, HStack, IconButton, Input, Stack, Switch, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { Plus, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import EntityDialog from "../../components/EntityDialog";
import FormField from "../../components/FormField";
import MoneyInput from "../../components/MoneyInput";
import type { Medicine, MedicineUnitInput } from "../../gen/inventory_iface/v1/medicine_pb";
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

// Larger (non-base) units edited as draft rows; the base unit is the form's
// `unit` + `unitPrice` fields.
type UnitDraft = { id: string; name: string; factor: string; sellPrice: string };

function toUnitInputs(units: UnitDraft[]): MedicineUnitInput[] {
  return units
    .filter((u) => u.name.trim() !== "")
    .map((u) => ({
      id: u.id,
      name: u.name.trim(),
      factor: BigInt(Math.trunc(Number(u.factor) || 0)),
      sellPrice: BigInt(Math.trunc(Number(u.sellPrice) || 0)),
      isBase: false,
      sellable: true,
      purchasable: true,
      sortOrder: 0,
      active: true,
    })) as MedicineUnitInput[];
}

function nonBaseDrafts(medicine: Medicine | null): UnitDraft[] {
  if (!medicine) return [];
  return medicine.units
    .filter((u) => !u.isBase)
    .map((u) => ({
      id: u.id,
      name: u.name,
      factor: String(u.factor),
      sellPrice: String(u.sellPrice),
    }));
}

function MedicineForm({
  form,
  units,
  setUnits,
  isCreate = true,
}: {
  form: ReturnType<typeof useForm<FormValues>>;
  units: UnitDraft[];
  setUnits: (u: UnitDraft[]) => void;
  isCreate?: boolean;
}) {
  const { t } = useTranslation();
  const baseName = form.watch("unit");
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
      <FormField
        control={form.control}
        name="unit"
        label={t("inventory.medicines.baseUnit")}
        required
      />
      <FormField
        control={form.control}
        name="unitPrice"
        label={t("inventory.medicines.basePrice")}
        money
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

      {/* Larger units (box / strip …) — converted to the base unit by factor. */}
      <Stack gap={2} borderTopWidth="1px" pt={3}>
        <Text fontWeight="medium" fontSize="sm">
          {t("inventory.medicines.unitsSection")}
        </Text>
        <Text fontSize="xs" color="fg.muted">
          {t("inventory.medicines.unitsBaseNote", { unit: baseName || "—" })}
        </Text>
        {units.length > 0 && (
          <HStack gap={2} fontSize="xs" color="fg.muted" px={1}>
            <Text flex="1">{t("inventory.medicines.unitName")}</Text>
            <Text width="90px">{t("inventory.medicines.unitFactor")}</Text>
            <Text width="120px">{t("inventory.medicines.unitSellPrice")}</Text>
            <Text width="32px" />
          </HStack>
        )}
        {units.map((u, i) => (
          <HStack key={i} gap={2}>
            <Input
              size="sm"
              flex="1"
              placeholder="box"
              value={u.name}
              onChange={(e) => setUnits(units.map((x, idx) => (idx === i ? { ...x, name: e.target.value } : x)))}
            />
            <Input
              size="sm"
              width="90px"
              type="number"
              inputMode="numeric"
              placeholder="100"
              value={u.factor}
              onChange={(e) => setUnits(units.map((x, idx) => (idx === i ? { ...x, factor: e.target.value } : x)))}
            />
            <MoneyInput
              size="sm"
              width="120px"
              value={u.sellPrice}
              onChange={(raw) => setUnits(units.map((x, idx) => (idx === i ? { ...x, sellPrice: raw } : x)))}
            />
            <IconButton
              aria-label="remove unit"
              size="sm"
              variant="ghost"
              onClick={() => setUnits(units.filter((_, idx) => idx !== i))}
            >
              <Trash2 size={14} />
            </IconButton>
          </HStack>
        ))}
        <Button
          size="xs"
          variant="outline"
          alignSelf="flex-start"
          onClick={() => setUnits([...units, { id: "", name: "", factor: "", sellPrice: "" }])}
        >
          <Plus size={14} />
          {t("inventory.medicines.addUnit")}
        </Button>
      </Stack>
    </Stack>
  );
}

export function CreateMedicineDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateMedicineMutation();
  const [units, setUnits] = useState<UnitDraft[]>([]);
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
      await create.mutateAsync({ ...values, units: toUnitInputs(units) });
      toast.success(t("common.create") + " ✓");
      form.reset();
      setUnits([]);
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
        <MedicineForm form={form} units={units} setUnits={setUnits} isCreate />
      </form>
    </EntityDialog>
  );
}

export function EditMedicineDialog({
  medicine,
  onClose,
}: {
  medicine: Medicine | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const update = useUpdateMedicineMutation();
  const [units, setUnits] = useState<UnitDraft[]>([]);
  // Re-seed the unit drafts whenever the edited medicine changes.
  useEffect(() => {
    setUnits(nonBaseDrafts(medicine));
  }, [medicine]);

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
        units: toUnitInputs(units),
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
        <MedicineForm form={form} units={units} setUnits={setUnits} />
      </form>
    </EntityDialog>
  );
}
