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
import { formatMoney } from "../../lib/format";
import { marginPct, priceFromMarkup } from "../../lib/pricing";
import { toast } from "../../lib/toaster";
import { useCreateMedicineMutation, useUpdateMedicineMutation } from "../../queries/medicines";

const Schema = z.object({
  sku: z.string().min(1),
  name: z.string().min(1),
  unit: z.string().min(1),
  unitPrice: z.coerce.bigint().min(0n),
  prescriptionRequired: z.boolean(),
});
type FormValues = z.infer<typeof Schema>;

// Larger (non-base) units edited as draft rows; the base unit is the form's
// `unit` + `unitPrice` fields. `markup` is a transient set-price helper (not sent).
type UnitDraft = { id: string; name: string; factor: string; sellPrice: string; markup: string };

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
      markup: "",
    }));
}

// MarkupMargin: a transient "set price from markup %" input + a live margin %
// readout, shown under a price field. Cost is the reference (latest) cost for
// this row (base = reference_cost; larger unit = reference_cost × factor).
function MarkupMargin({
  cost,
  sell,
  markup,
  onMarkup,
}: {
  cost: bigint;
  sell: number | bigint | string;
  markup: string;
  onMarkup: (markup: string) => void;
}) {
  const { t } = useTranslation();
  const m = marginPct(sell, cost);
  return (
    <HStack gap={2} fontSize="xs" color="fg.muted" pl={1}>
      <HStack gap={1}>
        <Text>{t("inventory.medicines.markup")}</Text>
        <Input
          size="xs"
          width="60px"
          type="number"
          inputMode="numeric"
          value={markup}
          onChange={(e) => onMarkup(e.target.value)}
        />
        <Text>%</Text>
      </HStack>
      <Text>
        {t("inventory.medicines.costLabel")} {formatMoney(cost)}
        {m != null ? ` · ${t("inventory.medicines.marginPct", { pct: m.toFixed(0) })}` : ""}
      </Text>
    </HStack>
  );
}

function MedicineForm({
  form,
  units,
  setUnits,
  referenceCost,
  isCreate = true,
}: {
  form: ReturnType<typeof useForm<FormValues>>;
  units: UnitDraft[];
  setUnits: (u: UnitDraft[]) => void;
  referenceCost: bigint;
  isCreate?: boolean;
}) {
  const { t } = useTranslation();
  const baseName = form.watch("unit");
  const [baseMarkup, setBaseMarkup] = useState("");
  const hasCost = referenceCost > 0n;
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
        name="unit"
        label={t("inventory.medicines.baseUnit")}
        required
      />
      <Stack gap={1}>
        <FormField
          control={form.control}
          name="unitPrice"
          label={t("inventory.medicines.basePrice")}
          money
          required
        />
        {hasCost && (
          <MarkupMargin
            cost={referenceCost}
            sell={form.watch("unitPrice")}
            markup={baseMarkup}
            onMarkup={(v) => {
              setBaseMarkup(v);
              form.setValue("unitPrice", priceFromMarkup(referenceCost, Number(v)));
            }}
          />
        )}
      </Stack>
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
        {units.map((u, i) => {
          const factor = BigInt(Math.trunc(Number(u.factor) || 0));
          const unitCost = referenceCost * factor;
          return (
            <Stack key={i} gap={1}>
              <HStack gap={2}>
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
              {hasCost && unitCost > 0n && (
                <MarkupMargin
                  cost={unitCost}
                  sell={u.sellPrice}
                  markup={u.markup}
                  onMarkup={(v) =>
                    setUnits(
                      units.map((x, idx) =>
                        idx === i
                          ? { ...x, markup: v, sellPrice: String(priceFromMarkup(unitCost, Number(v))) }
                          : x,
                      ),
                    )
                  }
                />
              )}
            </Stack>
          );
        })}
        <Button
          size="xs"
          variant="outline"
          alignSelf="flex-start"
          onClick={() => setUnits([...units, { id: "", name: "", factor: "", sellPrice: "", markup: "" }])}
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
        <MedicineForm form={form} units={units} setUnits={setUnits} referenceCost={0n} isCreate />
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
        <MedicineForm
          form={form}
          units={units}
          setUnits={setUnits}
          referenceCost={medicine?.referenceCost ?? 0n}
        />
      </form>
    </EntityDialog>
  );
}
