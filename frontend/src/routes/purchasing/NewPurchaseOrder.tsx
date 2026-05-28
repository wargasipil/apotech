import {
  Box,
  Button,
  Flex,
  HStack,
  Heading,
  IconButton,
  Input,
  Link as ChakraLink,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import DatePickerField from "../../components/DatePicker";
import EnumSelect from "../../components/EnumSelect";
import MoneyInput from "../../components/MoneyInput";
import SearchableSelect from "../../components/SearchableSelect";
import type { Medicine, MedicineUnit } from "../../gen/inventory_iface/v1/medicine_pb";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { searchMedicines } from "../../queries/medicines";
import { useCreatePurchaseOrderMutation } from "../../queries/purchasing";
import { searchSuppliers } from "../../queries/suppliers";

type Line = {
  medicineId: string;
  medicineUnitId: string; // chosen purchasable unit ("" => base)
  units: MedicineUnit[]; // purchasable + active units of the picked medicine
  orderedQty: number; // in the chosen unit
  lineTotal: number; // total cost for the line (Harga modal total); unit cost is derived
};

const factorOf = (l: Line): number => {
  const u = l.units.find((x) => x.id === l.medicineUnitId);
  return u ? Number(u.factor) : 1;
};
const baseQtyOf = (l: Line): number => l.orderedQty * factorOf(l);
// Cost per BASE unit is derived from the line total / base qty (rounded).
const unitCostOf = (l: Line): number => {
  const base = baseQtyOf(l);
  return base > 0 ? Math.round(l.lineTotal / base) : 0;
};
const unitNameOf = (l: Line): string =>
  l.units.find((x) => x.id === l.medicineUnitId)?.name ?? "";

const emptyLine = (): Line => ({
  medicineId: "",
  medicineUnitId: "",
  units: [],
  orderedQty: 1,
  lineTotal: 0,
});

export default function NewPurchaseOrder() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const createMut = useCreatePurchaseOrderMutation();

  const [supplierId, setSupplierId] = useState("");
  const [expectedAt, setExpectedAt] = useState("");
  const [note, setNote] = useState("");
  const [lines, setLines] = useState<Line[]>([emptyLine()]);

  const total = useMemo(() => lines.reduce((sum, l) => sum + l.lineTotal, 0), [lines]);

  const updateLine = (idx: number, patch: Partial<Line>) => {
    setLines((cur) => cur.map((l, i) => (i === idx ? { ...l, ...patch } : l)));
  };
  const removeLine = (idx: number) => setLines((cur) => cur.filter((_, i) => i !== idx));
  const addLine = () => setLines((cur) => [...cur, emptyLine()]);

  const onPickMedicine = (idx: number, m: Medicine | undefined) => {
    const units = (m?.units ?? []).filter((u) => u.purchasable && u.active);
    const base = units.find((u) => u.isBase);
    updateLine(idx, { units, medicineUnitId: base?.id ?? units[0]?.id ?? "" });
  };

  const canSubmit =
    !!supplierId &&
    lines.length > 0 &&
    lines.every((l) => l.medicineId && l.orderedQty > 0 && l.lineTotal >= 0);

  const submit = async () => {
    try {
      const res = await createMut.mutateAsync({
        supplierId,
        expectedAt,
        note,
        items: lines.map((l) => ({
          medicineId: l.medicineId,
          medicineUnitId: l.medicineUnitId,
          orderedQty: l.orderedQty,
          unitCostPrice: BigInt(unitCostOf(l)),
        })),
      });
      toast.success(t("common.create") + " ✓");
      if (res.order?.id) navigate(`/purchasing/${res.order.id}`);
      else navigate("/purchasing/all");
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={5}>
      <Heading size="md" mb={4}>
        {t("purchasing.newPo")}
      </Heading>
      <Stack gap={4}>
        <Flex gap={3} wrap="wrap">
          <Box flex="1" minW="240px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("purchasing.supplier")} *
            </Text>
            <SearchableSelect
              value={supplierId}
              onChange={setSupplierId}
              loadOptions={searchSuppliers}
              itemToString={(s) => `${s.code} · ${s.name}`}
              itemToValue={(s) => s.id}
              placeholder={t("purchasing.selectSupplier")}
            />
            <ChakraLink
              as="button"
              type="button"
              fontSize="xs"
              color="blue.500"
              mt={1}
              display="inline-flex"
              alignItems="center"
              gap={1}
              onClick={() => navigate("/inventory/suppliers")}
            >
              <Plus size={12} />
              {t("purchasing.addSupplierLink")}
            </ChakraLink>
          </Box>
          <Box flex="1" minW="200px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("purchasing.expectedAt")}
            </Text>
            <DatePickerField value={expectedAt} onChange={setExpectedAt} />
          </Box>
          <Box flex="2" minW="240px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("purchasing.note")}
            </Text>
            <Input value={note} onChange={(e) => setNote(e.target.value)} />
          </Box>
        </Flex>

        <Box>
          <HStack justify="space-between" mb={2}>
            <Heading size="sm">{t("purchasing.items")}</Heading>
            <Button size="xs" variant="outline" onClick={addLine}>
              <Plus size={14} />
              {t("purchasing.addLine")}
            </Button>
          </HStack>
          <Table.Root size="sm">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader minW="240px">{t("purchasing.selectMedicine")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.unit")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.qty")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.lineTotalInput")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.unitCostDerived")}</Table.ColumnHeader>
                <Table.ColumnHeader />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {lines.map((l, idx) => (
                <Table.Row key={idx}>
                  <Table.Cell>
                    <SearchableSelect
                      size="sm"
                      value={l.medicineId}
                      onChange={(v) => updateLine(idx, { medicineId: v })}
                      onSelectItem={(m) => onPickMedicine(idx, m)}
                      loadOptions={searchMedicines}
                      itemToString={(m) => `${m.sku} · ${m.name}`}
                      itemToValue={(m) => m.id}
                      placeholder={t("purchasing.selectMedicine")}
                    />
                  </Table.Cell>
                  <Table.Cell>
                    {l.units.length > 1 ? (
                      <EnumSelect
                        size="sm"
                        width="110px"
                        value={l.medicineUnitId}
                        onChange={(v) => updateLine(idx, { medicineUnitId: v })}
                        items={l.units}
                        itemToString={(u) => u.name}
                        itemToValue={(u) => u.id}
                      />
                    ) : (
                      <Text fontSize="sm" color="fg.muted">
                        {unitNameOf(l) || "—"}
                      </Text>
                    )}
                  </Table.Cell>
                  <Table.Cell>
                    <Input
                      size="sm"
                      type="number"
                      value={l.orderedQty}
                      onChange={(e) => updateLine(idx, { orderedQty: parseInt(e.target.value, 10) || 0 })}
                      w="80px"
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <MoneyInput
                      size="sm"
                      width="140px"
                      value={l.lineTotal}
                      onChange={(raw) => updateLine(idx, { lineTotal: Number(raw || 0) })}
                    />
                  </Table.Cell>
                  <Table.Cell fontFamily="mono" color="fg.muted">
                    {formatMoney(unitCostOf(l))}
                    {factorOf(l) > 1 && (
                      <Text fontSize="xs">
                        /{t("inventory.medicines.baseUnit").toLowerCase()}
                      </Text>
                    )}
                  </Table.Cell>
                  <Table.Cell>
                    <IconButton
                      aria-label="remove line"
                      size="xs"
                      variant="ghost"
                      onClick={() => removeLine(idx)}
                      disabled={lines.length === 1}
                    >
                      <Trash2 size={14} />
                    </IconButton>
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
          <HStack justify="flex-end" mt={2}>
            <Text fontWeight="semibold">{t("purchasing.subtotal")}:</Text>
            <Text fontWeight="semibold" fontFamily="mono">
              {formatMoney(total)}
            </Text>
          </HStack>
        </Box>

        <HStack justify="flex-end" gap={2} pt={2}>
          <Button variant="ghost" onClick={() => navigate("/purchasing/all")}>
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={submit}
            loading={createMut.isPending}
            disabled={!canSubmit}
          >
            {t("common.create")}
          </Button>
        </HStack>
      </Stack>
    </Box>
  );
}
