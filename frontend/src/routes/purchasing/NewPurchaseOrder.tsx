import {
  Box,
  Button,
  Flex,
  HStack,
  Heading,
  IconButton,
  Input,
  NativeSelect,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useMedicinesQuery } from "../../queries/medicines";
import { useCreatePurchaseOrderMutation } from "../../queries/purchasing";
import { useSuppliersQuery } from "../../queries/suppliers";

type Line = {
  medicineId: string;
  orderedQty: number;
  unitCostPrice: number;
};

export default function NewPurchaseOrder() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const suppliersQ = useSuppliersQuery(false);
  const medicinesQ = useMedicinesQuery(false);
  const createMut = useCreatePurchaseOrderMutation();

  const [supplierId, setSupplierId] = useState("");
  const [expectedAt, setExpectedAt] = useState("");
  const [note, setNote] = useState("");
  const [lines, setLines] = useState<Line[]>([{ medicineId: "", orderedQty: 1, unitCostPrice: 0 }]);

  const total = useMemo(
    () => lines.reduce((sum, l) => sum + l.orderedQty * l.unitCostPrice, 0),
    [lines],
  );

  const updateLine = (idx: number, patch: Partial<Line>) => {
    setLines((cur) => cur.map((l, i) => (i === idx ? { ...l, ...patch } : l)));
  };
  const removeLine = (idx: number) => setLines((cur) => cur.filter((_, i) => i !== idx));
  const addLine = () =>
    setLines((cur) => [...cur, { medicineId: "", orderedQty: 1, unitCostPrice: 0 }]);

  const canSubmit =
    !!supplierId &&
    lines.length > 0 &&
    lines.every((l) => l.medicineId && l.orderedQty > 0 && l.unitCostPrice >= 0);

  const submit = async () => {
    try {
      const res = await createMut.mutateAsync({
        supplierId,
        expectedAt,
        note,
        items: lines.map((l) => ({
          medicineId: l.medicineId,
          orderedQty: l.orderedQty,
          unitCostPrice: BigInt(l.unitCostPrice),
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
            <NativeSelect.Root>
              <NativeSelect.Field value={supplierId} onChange={(e) => setSupplierId(e.target.value)}>
                <option value="">{t("purchasing.selectSupplier")}</option>
                {(suppliersQ.data ?? []).map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </NativeSelect.Field>
              <NativeSelect.Indicator />
            </NativeSelect.Root>
          </Box>
          <Box flex="1" minW="200px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("purchasing.expectedAt")}
            </Text>
            <Input type="date" value={expectedAt} onChange={(e) => setExpectedAt(e.target.value)} />
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
                <Table.ColumnHeader>{t("purchasing.qty")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.unitCost")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.lineTotal")}</Table.ColumnHeader>
                <Table.ColumnHeader />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {lines.map((l, idx) => (
                <Table.Row key={idx}>
                  <Table.Cell>
                    <NativeSelect.Root size="sm">
                      <NativeSelect.Field
                        value={l.medicineId}
                        onChange={(e) => updateLine(idx, { medicineId: e.target.value })}
                      >
                        <option value="">—</option>
                        {(medicinesQ.data ?? []).map((m) => (
                          <option key={m.id} value={m.id}>
                            {m.sku} · {m.name}
                          </option>
                        ))}
                      </NativeSelect.Field>
                      <NativeSelect.Indicator />
                    </NativeSelect.Root>
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
                    <Input
                      size="sm"
                      type="number"
                      value={l.unitCostPrice}
                      onChange={(e) =>
                        updateLine(idx, { unitCostPrice: parseInt(e.target.value, 10) || 0 })
                      }
                      w="120px"
                    />
                  </Table.Cell>
                  <Table.Cell fontFamily="mono">
                    {formatMoney(l.orderedQty * l.unitCostPrice)}
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
