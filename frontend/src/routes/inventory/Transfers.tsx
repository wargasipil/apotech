import { useState } from "react";
import {
  Box,
  Button,
  HStack,
  IconButton,
  Input,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { ArrowRight, Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import EntityDrawer from "../../components/EntityDrawer";
import EnumSelect from "../../components/EnumSelect";
import Pagination from "../../components/Pagination";
import SearchableSelect from "../../components/SearchableSelect";
import { searchBatches } from "../../queries/batches";
import { formatUnix } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import { useWarehousesQuery } from "../../queries/warehouses";
import { useCreateTransferMutation, useTransfersQuery } from "../../queries/transfers";

type LineDraft = { batchId: string; label: string; qty: string };

export default function Transfers() {
  const { t } = useTranslation();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const { page, setPage, pageSize, setPageSize } = usePageState("");
  const transfersQ = useTransfersQuery({ page, pageSize });

  return (
    <Stack gap={4}>
      <HStack justify="space-between">
        <Text fontSize="sm" color="fg.muted">
          {t("transfers.intro")}
        </Text>
        <Button size="sm" colorPalette="blue" onClick={() => setDrawerOpen(true)}>
          <Plus size={16} />
          {t("transfers.newTransfer")}
        </Button>
      </HStack>

      {transfersQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("transfers.transferNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("transfers.route")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("transfers.lines")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("transfers.note")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("transfers.when")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {transfersQ.rows.map((tr) => (
              <Table.Row key={tr.id}>
                <Table.Cell fontFamily="mono">{tr.transferNo}</Table.Cell>
                <Table.Cell>
                  <HStack gap={1}>
                    <Text>{tr.fromWarehouseName}</Text>
                    <ArrowRight size={14} />
                    <Text>{tr.toWarehouseName}</Text>
                  </HStack>
                </Table.Cell>
                <Table.Cell>{tr.lines.length}</Table.Cell>
                <Table.Cell>{tr.note}</Table.Cell>
                <Table.Cell>{formatUnix(tr.createdAt)}</Table.Cell>
              </Table.Row>
            ))}
            {transfersQ.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={5}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("transfers.empty")}
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
        total={transfersQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <CreateDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </Stack>
  );
}

function CreateDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const warehousesQ = useWarehousesQuery();
  const create = useCreateTransferMutation();
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [note, setNote] = useState("");
  const [lines, setLines] = useState<LineDraft[]>([{ batchId: "", label: "", qty: "" }]);

  const reset = () => {
    setFrom("");
    setTo("");
    setNote("");
    setLines([{ batchId: "", label: "", qty: "" }]);
  };

  const validLines = lines.filter((l) => l.batchId && Number(l.qty) > 0);
  const canSubmit = from !== "" && to !== "" && from !== to && validLines.length > 0;

  const submit = async () => {
    try {
      await create.mutateAsync({
        fromWarehouseId: from,
        toWarehouseId: to,
        note,
        lines: validLines.map((l) => ({ batchId: l.batchId, qty: Number(l.qty) })),
      });
      toast.success(t("transfers.created"));
      reset();
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  const warehouseItems = warehousesQ.data ?? [];

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("transfers.newTransfer")}
      footer={
        <HStack justify="space-between" w="100%">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending} disabled={!canSubmit}>
            {t("transfers.submit")}
          </Button>
        </HStack>
      }
    >
      <Stack gap={4}>
        <Stack gap={1}>
          <Text fontSize="sm" fontWeight="medium" color="fg.muted">
            {t("transfers.from")} *
          </Text>
          <EnumSelect
            value={from}
            onChange={setFrom}
            items={warehouseItems}
            itemToString={(w) => `${w.code} · ${w.name}`}
            itemToValue={(w) => w.id}
            placeholder={t("transfers.selectWarehouse")}
          />
        </Stack>
        <Stack gap={1}>
          <Text fontSize="sm" fontWeight="medium" color="fg.muted">
            {t("transfers.to")} *
          </Text>
          <EnumSelect
            value={to}
            onChange={setTo}
            items={warehouseItems.filter((w) => w.id !== from)}
            itemToString={(w) => `${w.code} · ${w.name}`}
            itemToValue={(w) => w.id}
            placeholder={t("transfers.selectWarehouse")}
          />
        </Stack>

        <Stack gap={2}>
          <Text fontSize="sm" fontWeight="medium" color="fg.muted">
            {t("transfers.items")} *
          </Text>
          {lines.map((line, idx) => (
            <HStack key={idx} gap={2} align="flex-start">
              <Box flex="1">
                <SearchableSelect
                  value={line.batchId}
                  onChange={(v) =>
                    setLines((ls) => ls.map((l, i) => (i === idx ? { ...l, batchId: v } : l)))
                  }
                  loadOptions={(q) => searchBatches(q)}
                  itemToString={(b) =>
                    `${b.batchNumber || b.id.slice(0, 8)} (${String(b.currentQuantity)})`
                  }
                  itemToValue={(b) => b.id}
                  selectedLabel={line.label || undefined}
                  placeholder={t("transfers.pickBatch")}
                />
              </Box>
              <Input
                type="number"
                min={1}
                width="90px"
                value={line.qty}
                placeholder={t("transfers.qty")}
                onChange={(e) =>
                  setLines((ls) => ls.map((l, i) => (i === idx ? { ...l, qty: e.target.value } : l)))
                }
              />
              <IconButton
                aria-label={t("transfers.removeLine")}
                size="sm"
                variant="ghost"
                disabled={lines.length <= 1}
                onClick={() => setLines((ls) => ls.filter((_, i) => i !== idx))}
              >
                <Trash2 size={14} />
              </IconButton>
            </HStack>
          ))}
          <Button
            size="xs"
            variant="outline"
            alignSelf="flex-start"
            onClick={() => setLines((ls) => [...ls, { batchId: "", label: "", qty: "" }])}
          >
            <Plus size={14} />
            {t("transfers.addLine")}
          </Button>
        </Stack>

        <Stack gap={1}>
          <Text fontSize="sm" fontWeight="medium" color="fg.muted">
            {t("transfers.note")}
          </Text>
          <Input value={note} onChange={(e) => setNote(e.target.value)} />
        </Stack>
      </Stack>
    </EntityDrawer>
  );
}
