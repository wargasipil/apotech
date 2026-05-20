import { useMemo, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Dialog,
  HStack,
  Heading,
  IconButton,
  Input,
  Portal,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Ban, Check, Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";

import EnumSelect from "../../components/EnumSelect";
import SearchableSelect from "../../components/SearchableSelect";
import type { StocktakeLine } from "../../gen/stocktake_iface/v1/stocktake_pb";
import { searchBatches } from "../../queries/batches";
import { searchMedicines } from "../../queries/medicines";
import { toast } from "../../lib/toaster";
import {
  useAddAllInStockBatchesMutation,
  useAddBatchesMutation,
  useCompleteStocktakeMutation,
  useRecordCountMutation,
  useRemoveLineMutation,
  useSetLineDispositionMutation,
  useStocktakeQuery,
  useVoidStocktakeMutation,
} from "../../queries/stocktake";

const WRITE_OFF_KINDS = ["EXPIRED", "DAMAGED", "LOST", "THEFT", "OTHER"] as const;

function statusPalette(status: string): string {
  switch (status) {
    case "DRAFT":
      return "yellow";
    case "COMPLETED":
      return "green";
    case "VOIDED":
      return "gray";
    default:
      return "gray";
  }
}

export default function StocktakeDetail() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const q = useStocktakeQuery(id);

  const addMut = useAddBatchesMutation(id);
  const addAllMut = useAddAllInStockBatchesMutation(id);
  const completeMut = useCompleteStocktakeMutation(id);
  const voidMut = useVoidStocktakeMutation(id);

  const [addOpen, setAddOpen] = useState(false);

  if (q.isLoading || !q.data) {
    return (
      <Box p={6} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  const session = q.data.session;
  const lines = q.data.lines ?? [];
  const isDraft = session?.status === "DRAFT";

  const onComplete = async () => {
    try {
      const res = await completeMut.mutateAsync({ sessionId: id });
      toast.success(
        t("inventory.stocktake.completed", { count: res.movementsWritten }),
      );
    } catch {
      /* toast handled globally */
    }
  };

  const onVoid = async () => {
    try {
      await voidMut.mutateAsync({ sessionId: id });
      toast.success(t("inventory.stocktake.voided"));
    } catch {
      /* */
    }
  };

  const onAddAll = async () => {
    try {
      const res = await addAllMut.mutateAsync({ sessionId: id });
      toast.success(
        t("inventory.stocktake.addedSummary", {
          added: res.addedCount,
          skipped: res.skippedCount,
        }),
      );
    } catch {
      /* */
    }
  };

  return (
    <Stack gap={4}>
      <HStack justify="space-between" align="flex-start">
        <Stack gap={1}>
          <HStack gap={3}>
            <Heading size="md">{session?.name || session?.id.slice(0, 8)}</Heading>
            <Badge colorPalette={statusPalette(session?.status ?? "")}>
              {t(
                `inventory.stocktake.statuses.${(session?.status ?? "").toLowerCase()}`,
                session?.status ?? "",
              )}
            </Badge>
          </HStack>
          <Text fontSize="sm" color="fg.muted">
            {t("inventory.stocktake.summary", {
              total: session?.lineCount ?? 0,
              counted: session?.countedCount ?? 0,
              variances: session?.varianceCount ?? 0,
            })}
          </Text>
        </Stack>
        {isDraft && (
          <HStack gap={2}>
            <Button size="sm" variant="outline" onClick={() => setAddOpen(true)}>
              <Plus size={16} />
              {t("inventory.stocktake.addBatches")}
            </Button>
            <Button size="sm" variant="outline" onClick={onAddAll} loading={addAllMut.isPending}>
              {t("inventory.stocktake.addAllInStock")}
            </Button>
            <Button
              size="sm"
              colorPalette="red"
              variant="ghost"
              onClick={onVoid}
              loading={voidMut.isPending}
            >
              <Ban size={16} />
              {t("inventory.stocktake.void")}
            </Button>
            <Button
              size="sm"
              colorPalette="blue"
              onClick={onComplete}
              loading={completeMut.isPending}
            >
              <Check size={16} />
              {t("inventory.stocktake.complete")}
            </Button>
          </HStack>
        )}
      </HStack>

      <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
        <Table.Header bg="bg.muted">
          <Table.Row>
            <Table.ColumnHeader>{t("inventory.stocktake.medicine")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("inventory.stocktake.batch")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("inventory.stocktake.expiry")}</Table.ColumnHeader>
            <Table.ColumnHeader textAlign="right">
              {t("inventory.stocktake.expected")}
            </Table.ColumnHeader>
            <Table.ColumnHeader textAlign="right">
              {t("inventory.stocktake.countedQty")}
            </Table.ColumnHeader>
            <Table.ColumnHeader textAlign="right">
              {t("inventory.stocktake.variance")}
            </Table.ColumnHeader>
            <Table.ColumnHeader>{t("inventory.stocktake.disposition")}</Table.ColumnHeader>
            {isDraft && <Table.ColumnHeader />}
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {lines.map((l) => (
            <LineRow key={l.id} line={l} sessionId={id} editable={isDraft} />
          ))}
          {lines.length === 0 && (
            <Table.Row>
              <Table.Cell colSpan={isDraft ? 8 : 7}>
                <Text color="fg.muted" textAlign="center" py={4}>
                  {t("inventory.stocktake.sessionEmpty")}
                </Text>
              </Table.Cell>
            </Table.Row>
          )}
        </Table.Body>
      </Table.Root>

      <AddBatchesDialog
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onAdd={async (batchIds) => {
          try {
            const res = await addMut.mutateAsync({ sessionId: id, batchIds });
            toast.success(
              t("inventory.stocktake.addedSummary", {
                added: res.addedCount,
                skipped: res.skippedCount,
              }),
            );
            setAddOpen(false);
          } catch {
            /* */
          }
        }}
        loading={addMut.isPending}
      />
    </Stack>
  );
}

function LineRow({
  line,
  sessionId,
  editable,
}: {
  line: StocktakeLine;
  sessionId: string;
  editable: boolean;
}) {
  const { t } = useTranslation();
  const recordMut = useRecordCountMutation(sessionId);
  const setDispMut = useSetLineDispositionMutation(sessionId);
  const removeMut = useRemoveLineMutation(sessionId);

  const [countedStr, setCountedStr] = useState<string>(
    line.counted ? String(line.countedQty) : "",
  );

  const persistCount = async () => {
    const trimmed = countedStr.trim();
    if (trimmed === "") return;
    const n = Number(trimmed);
    if (!Number.isFinite(n) || n < 0 || !Number.isInteger(n)) {
      toast.error(t("inventory.stocktake.invalidCount"));
      return;
    }
    if (line.counted && n === line.countedQty) return;
    try {
      await recordMut.mutateAsync({ lineId: line.id, countedQty: n });
    } catch {
      /* */
    }
  };

  const variance = line.counted ? line.countedQty - line.expectedQty : 0;
  const isNegative = line.counted && variance < 0;

  const setDisposition = async (disposition: string, kind: string, note: string) => {
    try {
      await setDispMut.mutateAsync({
        lineId: line.id,
        disposition,
        writeOffKind: kind,
        dispositionNote: note,
      });
    } catch {
      /* */
    }
  };

  return (
    <Table.Row>
      <Table.Cell>
        <Stack gap={0}>
          <Text>{line.medicineName || line.medicineId.slice(0, 8)}</Text>
          {line.medicineSku && (
            <Text fontSize="xs" color="fg.muted">
              {line.medicineSku}
            </Text>
          )}
        </Stack>
      </Table.Cell>
      <Table.Cell>{line.batchNumber || line.batchId.slice(0, 8)}</Table.Cell>
      <Table.Cell>{line.expiryDate}</Table.Cell>
      <Table.Cell textAlign="right">{line.expectedQty}</Table.Cell>
      <Table.Cell textAlign="right">
        {editable ? (
          <Input
            size="sm"
            type="number"
            min={0}
            value={countedStr}
            onChange={(e) => setCountedStr(e.target.value)}
            onBlur={persistCount}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                (e.target as HTMLInputElement).blur();
              }
            }}
            width="80px"
            textAlign="right"
          />
        ) : line.counted ? (
          line.countedQty
        ) : (
          "—"
        )}
      </Table.Cell>
      <Table.Cell textAlign="right">
        {line.counted ? (
          <Text
            color={variance === 0 ? "fg.muted" : variance > 0 ? "green.solid" : "red.solid"}
            fontWeight={variance === 0 ? "normal" : "medium"}
          >
            {variance > 0 ? `+${variance}` : variance}
          </Text>
        ) : (
          <Text color="fg.muted">—</Text>
        )}
      </Table.Cell>
      <Table.Cell>
        {isNegative && editable ? (
          <DispositionPicker
            disposition={line.disposition}
            kind={line.writeOffKind}
            note={line.dispositionNote}
            onChange={setDisposition}
          />
        ) : isNegative ? (
          <Text fontSize="xs">
            {t(`inventory.stocktake.dispositions.${line.disposition.toLowerCase()}`, line.disposition)}
            {line.writeOffKind ? ` · ${t(`inventory.stocktake.writeOffKinds.${line.writeOffKind.toLowerCase()}`, line.writeOffKind)}` : ""}
          </Text>
        ) : (
          <Text color="fg.muted">—</Text>
        )}
      </Table.Cell>
      {editable && (
        <Table.Cell>
          <IconButton
            aria-label={t("inventory.stocktake.removeLine")}
            size="xs"
            variant="ghost"
            onClick={() => removeMut.mutate({ lineId: line.id })}
            loading={removeMut.isPending}
          >
            <Trash2 size={14} />
          </IconButton>
        </Table.Cell>
      )}
    </Table.Row>
  );
}

function DispositionPicker({
  disposition,
  kind,
  note,
  onChange,
}: {
  disposition: string;
  kind: string;
  note: string;
  onChange: (disposition: string, kind: string, note: string) => void;
}) {
  const { t } = useTranslation();
  const [localKind, setLocalKind] = useState(kind);
  const [localNote, setLocalNote] = useState(note);

  return (
    <Stack gap={1} minW="240px">
      <EnumSelect
        size="xs"
        value={disposition}
        onChange={(v) =>
          onChange(v, v === "WRITE_OFF" ? localKind || "EXPIRED" : "", localNote)
        }
        items={[
          { value: "ADJUSTMENT", label: t("inventory.stocktake.dispositions.adjustment") },
          { value: "WRITE_OFF", label: t("inventory.stocktake.dispositions.writeOff") },
        ]}
        itemToString={(o) => o.label}
        itemToValue={(o) => o.value}
      />
      {disposition === "WRITE_OFF" && (
        <>
          <EnumSelect
            size="xs"
            value={localKind || ""}
            onChange={(v) => {
              setLocalKind(v);
              onChange(disposition, v, localNote);
            }}
            items={WRITE_OFF_KINDS.map((k) => ({
              value: k,
              label: t(`inventory.stocktake.writeOffKinds.${k.toLowerCase()}`, k),
            }))}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
            placeholder={t("inventory.stocktake.selectKind")}
          />
          <Input
            size="xs"
            value={localNote}
            placeholder={t("inventory.stocktake.notePlaceholder")}
            onChange={(e) => setLocalNote(e.target.value)}
            onBlur={() => onChange(disposition, localKind, localNote)}
          />
        </>
      )}
    </Stack>
  );
}

function AddBatchesDialog({
  open,
  onClose,
  onAdd,
  loading,
}: {
  open: boolean;
  onClose: () => void;
  onAdd: (batchIds: string[]) => void;
  loading: boolean;
}) {
  const { t } = useTranslation();
  const [medicineId, setMedicineId] = useState("");
  const [batchId, setBatchId] = useState("");

  const submit = () => {
    if (!batchId) return;
    onAdd([batchId]);
    setBatchId("");
    setMedicineId("");
  };

  const loadBatches = useMemo(
    () => (q: string) => searchBatches(q, medicineId || undefined),
    [medicineId],
  );

  return (
    <Dialog.Root open={open} onOpenChange={(d) => (!d.open ? onClose() : null)}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("inventory.stocktake.addBatches")}</Dialog.Title>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                <Stack gap={1}>
                  <Text fontSize="sm" color="fg.muted">
                    {t("inventory.stocktake.filterByMedicine")}
                  </Text>
                  <SearchableSelect
                    value={medicineId}
                    onChange={(v) => {
                      setMedicineId(v);
                      setBatchId("");
                    }}
                    loadOptions={searchMedicines}
                    itemToString={(m) => `${m.name} · ${m.sku}`}
                    itemToValue={(m) => m.id}
                    placeholder={t("inventory.stocktake.anyMedicine")}
                  />
                </Stack>
                <Stack gap={1}>
                  <Text fontSize="sm" color="fg.muted">
                    {t("inventory.stocktake.batch")} *
                  </Text>
                  <SearchableSelect
                    value={batchId}
                    onChange={setBatchId}
                    loadOptions={loadBatches}
                    itemToString={(b) =>
                      `${b.batchNumber || b.id.slice(0, 8)} (qty ${String(b.currentQuantity)})`
                    }
                    itemToValue={(b) => b.id}
                    placeholder={t("inventory.stocktake.pickBatch")}
                  />
                </Stack>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <Button variant="ghost" onClick={onClose}>
                {t("common.cancel")}
              </Button>
              <Button
                colorPalette="blue"
                onClick={submit}
                loading={loading}
                disabled={!batchId}
              >
                {t("inventory.stocktake.add")}
              </Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
