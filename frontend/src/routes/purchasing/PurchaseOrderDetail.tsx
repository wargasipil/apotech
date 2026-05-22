import {
  Badge,
  Box,
  Button,
  Dialog,
  Flex,
  Grid,
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
import { Ban, DollarSign, PackageCheck, Send, X } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import BackButton from "../../components/BackButton";
import DatePickerField from "../../components/DatePicker";
import {
  POStatus,
  type PurchaseOrder,
  type PurchaseOrderItem,
} from "../../gen/purchasing_iface/v1/order_pb";
import { formatDate, formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useAllMedicinesQuery } from "../../queries/medicines";
import {
  useCreateReceiptMutation,
  usePayPurchaseMutation,
  usePurchaseOrderQuery,
  useReceiptsQuery,
  useSendPurchaseOrderMutation,
  useVoidPurchaseOrderMutation,
} from "../../queries/purchasing";
import { useAllSuppliersQuery } from "../../queries/suppliers";

const STATUS_PALETTE: Record<POStatus, string> = {
  [POStatus.PO_STATUS_UNSPECIFIED]: "gray",
  [POStatus.PO_STATUS_DRAFT]: "gray",
  [POStatus.PO_STATUS_SENT]: "blue",
  [POStatus.PO_STATUS_PARTIALLY_RECEIVED]: "orange",
  [POStatus.PO_STATUS_RECEIVED]: "green",
  [POStatus.PO_STATUS_CLOSED]: "green",
  [POStatus.PO_STATUS_VOIDED]: "red",
};

export default function PurchaseOrderDetail() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id = "" } = useParams();

  const poQ = usePurchaseOrderQuery(id);
  const suppliersQ = useAllSuppliersQuery(true);
  const medicinesQ = useAllMedicinesQuery(true);
  const receiptsQ = useReceiptsQuery({ purchaseOrderId: id });

  const sendMut = useSendPurchaseOrderMutation();
  const voidMut = useVoidPurchaseOrderMutation();

  const [receiveOpen, setReceiveOpen] = useState(false);
  const [payOpen, setPayOpen] = useState(false);

  const supplierName = useMemo(
    () => new Map(suppliersQ.rows.map((s) => [s.id, s.name])),
    [suppliersQ.rows],
  );
  const medicineName = useMemo(
    () => new Map(medicinesQ.rows.map((m) => [m.id, m.name])),
    [medicinesQ.rows],
  );

  if (poQ.isLoading || !poQ.data) {
    return (
      <Box p={6} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  const po = poQ.data;

  const canSend = po.status === POStatus.PO_STATUS_DRAFT && po.items.length > 0;
  const canVoid = po.status === POStatus.PO_STATUS_DRAFT || po.status === POStatus.PO_STATUS_SENT;
  const canReceive =
    po.status === POStatus.PO_STATUS_SENT || po.status === POStatus.PO_STATUS_PARTIALLY_RECEIVED;
  const canPay = po.status !== POStatus.PO_STATUS_VOIDED && po.status !== POStatus.PO_STATUS_DRAFT;

  const onSend = async () => {
    try {
      await sendMut.mutateAsync(id);
      toast.success(t("purchasing.actions.send") + " ✓");
    } catch {
      /* */
    }
  };
  const onVoid = async () => {
    try {
      await voidMut.mutateAsync(id);
      toast.success(t("purchasing.actions.void") + " ✓");
    } catch {
      /* */
    }
  };

  return (
    <Stack gap={6}>
      <BackButton to="/purchasing" />
      <Flex justify="space-between" align="center" wrap="wrap" gap={2}>
        <HStack gap={3}>
          <Heading size="md" fontFamily="mono">
            {po.poNo || po.id.slice(0, 8)}
          </Heading>
          <Badge colorPalette={STATUS_PALETTE[po.status]}>
            {t(`purchasing.states.${statusKey(po.status)}`)}
          </Badge>
        </HStack>
        <HStack gap={2}>
          {canSend && (
            <Button size="sm" colorPalette="blue" onClick={onSend} loading={sendMut.isPending}>
              <Send size={14} />
              {t("purchasing.actions.send")}
            </Button>
          )}
          {canReceive && (
            <Button size="sm" colorPalette="blue" onClick={() => setReceiveOpen(true)}>
              <PackageCheck size={14} />
              {t("purchasing.actions.receive")}
            </Button>
          )}
          {canPay && (
            <Button size="sm" variant="outline" onClick={() => setPayOpen(true)}>
              <DollarSign size={14} />
              {t("purchasing.actions.pay")}
            </Button>
          )}
          {canVoid && (
            <Button size="sm" variant="outline" colorPalette="red" onClick={onVoid}>
              <Ban size={14} />
              {t("purchasing.actions.void")}
            </Button>
          )}
        </HStack>
      </Flex>

      {/* Header info */}
      <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
        <Grid templateColumns={{ base: "1fr", md: "repeat(4, 1fr)" }} gap={4}>
          <Info label={t("purchasing.supplier")} value={supplierName.get(po.supplierId) ?? po.supplierId} />
          <Info label={t("purchasing.expectedAt")} value={po.expectedAt ? formatDate(po.expectedAt) : "—"} />
          <Info
            label={t("purchasing.totalOrdered")}
            value={formatMoney(Number(po.orderedTotal))}
          />
          <Info
            label={t("purchasing.outstanding")}
            value={formatMoney(Number(po.outstanding))}
            highlight={po.outstanding > 0n}
          />
        </Grid>
        {po.note && (
          <Box mt={3} pt={3} borderTopWidth="1px">
            <Text fontSize="xs" color="fg.muted">
              {t("purchasing.note")}
            </Text>
            <Text fontSize="sm">{po.note}</Text>
          </Box>
        )}
      </Box>

      {/* Lines */}
      <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
        <Heading size="sm" mb={3}>
          {t("purchasing.items")}
        </Heading>
        <Table.Root size="sm">
          <Table.Header>
            <Table.Row>
              <Table.ColumnHeader>{t("purchasing.selectMedicine")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.ordered")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.received")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.unitCost")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.lineTotal")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {po.items.map((it) => (
              <Table.Row key={it.id}>
                <Table.Cell>{medicineName.get(it.medicineId) ?? it.medicineId.slice(0, 8)}</Table.Cell>
                <Table.Cell>{it.orderedQty}</Table.Cell>
                <Table.Cell>
                  {it.receivedQty} / {it.orderedQty}
                </Table.Cell>
                <Table.Cell fontFamily="mono">{formatMoney(Number(it.unitCostPrice))}</Table.Cell>
                <Table.Cell fontFamily="mono">{formatMoney(Number(it.subtotal))}</Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Root>
      </Box>

      {/* Receipts */}
      <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
        <Heading size="sm" mb={3}>
          {t("purchasing.receipt")}s
        </Heading>
        {receiptsQ.isLoading ? (
          <Spinner size="sm" />
        ) : (receiptsQ.data?.length ?? 0) === 0 ? (
          <Text fontSize="sm" color="fg.muted">
            {t("purchasing.noReceipts")}
          </Text>
        ) : (
          <Stack gap={3}>
            {receiptsQ.data!.map((r) => (
              <Box key={r.id} borderWidth="1px" borderRadius="md" p={3}>
                <HStack justify="space-between" mb={2}>
                  <HStack gap={3}>
                    <Text fontFamily="mono" fontWeight="medium">
                      {r.receiptNo}
                    </Text>
                    {r.invoiceNo && (
                      <Text fontSize="sm" color="fg.muted">
                        {t("purchasing.invoiceNo")}: {r.invoiceNo}
                      </Text>
                    )}
                  </HStack>
                  <Text fontSize="sm" color="fg.muted">
                    {formatDate(r.receivedAt)}
                  </Text>
                </HStack>
                <Table.Root size="sm">
                  <Table.Header>
                    <Table.Row>
                      <Table.ColumnHeader>{t("purchasing.selectMedicine")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("purchasing.qty")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("purchasing.batchNumber")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("purchasing.expiryDate")}</Table.ColumnHeader>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {r.items.map((it) => (
                      <Table.Row key={it.id}>
                        <Table.Cell>{medicineName.get(it.medicineId) ?? it.medicineId.slice(0, 8)}</Table.Cell>
                        <Table.Cell>{it.qty}</Table.Cell>
                        <Table.Cell>{it.batchNumber || "—"}</Table.Cell>
                        <Table.Cell>{formatDate(it.expiryDate)}</Table.Cell>
                      </Table.Row>
                    ))}
                  </Table.Body>
                </Table.Root>
              </Box>
            ))}
          </Stack>
        )}
      </Box>

      <ReceiveDialog
        open={receiveOpen}
        onClose={() => setReceiveOpen(false)}
        po={po}
        medicineName={medicineName}
      />
      <PayDialog open={payOpen} onClose={() => setPayOpen(false)} poId={po.id} outstanding={Number(po.outstanding)} />

      <Button variant="ghost" alignSelf="flex-start" onClick={() => navigate("/purchasing/all")}>
        ← {t("purchasing.title")}
      </Button>
    </Stack>
  );
}

function Info({
  label,
  value,
  highlight,
}: {
  label: string;
  value: string;
  highlight?: boolean;
}) {
  return (
    <Box>
      <Text fontSize="xs" color="fg.muted">
        {label}
      </Text>
      <Text fontSize="md" fontWeight="medium" color={highlight ? "fg.error" : "fg"} fontFamily="mono">
        {value}
      </Text>
    </Box>
  );
}

function ReceiveDialog({
  open,
  onClose,
  po,
  medicineName,
}: {
  open: boolean;
  onClose: () => void;
  po: PurchaseOrder;
  medicineName: Map<string, string>;
}) {
  const { t } = useTranslation();
  const createReceipt = useCreateReceiptMutation();
  const today = new Date().toISOString().slice(0, 10);
  const [receivedAt, setReceivedAt] = useState(today);
  const [note, setNote] = useState("");
  const [invoiceNo, setInvoiceNo] = useState("");

  type LineDraft = {
    purchaseOrderItemId: string;
    qty: number;
    unitCostPrice: number;
    batchNumber: string;
    expiryDate: string;
    remaining: number;
  };
  const initialLines = (items: PurchaseOrderItem[]): LineDraft[] =>
    items
      .filter((it) => it.receivedQty < it.orderedQty)
      .map((it) => ({
        purchaseOrderItemId: it.id,
        qty: it.orderedQty - it.receivedQty,
        unitCostPrice: Number(it.unitCostPrice),
        batchNumber: "",
        expiryDate: "",
        remaining: it.orderedQty - it.receivedQty,
      }));
  const [lines, setLines] = useState<LineDraft[]>(() => initialLines(po.items));

  // Reset when opening for a different PO.
  if (open && lines.length === 0 && po.items.some((it) => it.receivedQty < it.orderedQty)) {
    setLines(initialLines(po.items));
  }

  const canSubmit =
    lines.length > 0 &&
    lines.every((l) => l.qty > 0 && l.qty <= l.remaining && l.expiryDate);

  const submit = async () => {
    try {
      await createReceipt.mutateAsync({
        purchaseOrderId: po.id,
        receivedAt,
        note,
        invoiceNo,
        lines: lines
          .filter((l) => l.qty > 0)
          .map((l) => ({
            purchaseOrderItemId: l.purchaseOrderItemId,
            qty: l.qty,
            unitCostPrice: BigInt(l.unitCostPrice),
            batchNumber: l.batchNumber,
            expiryDate: l.expiryDate,
          })),
      });
      toast.success(t("purchasing.receipt") + " ✓");
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  const updateLine = (idx: number, patch: Partial<LineDraft>) =>
    setLines((cur) => cur.map((l, i) => (i === idx ? { ...l, ...patch } : l)));

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()} size="xl">
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("purchasing.receiveTitle")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={4}>
                <HStack gap={3}>
                  <Box flex="1">
                    <Text fontSize="xs" color="fg.muted">
                      {t("purchasing.receivedAt")}
                    </Text>
                    <DatePickerField value={receivedAt} onChange={setReceivedAt} />
                  </Box>
                  <Box flex="1">
                    <Text fontSize="xs" color="fg.muted">
                      {t("purchasing.invoiceNo")}
                    </Text>
                    <Input value={invoiceNo} onChange={(e) => setInvoiceNo(e.target.value)} />
                  </Box>
                  <Box flex="2">
                    <Text fontSize="xs" color="fg.muted">
                      {t("purchasing.note")}
                    </Text>
                    <Input value={note} onChange={(e) => setNote(e.target.value)} />
                  </Box>
                </HStack>

                <Table.Root size="sm">
                  <Table.Header>
                    <Table.Row>
                      <Table.ColumnHeader>{t("purchasing.selectMedicine")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("purchasing.qty")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("purchasing.unitCost")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("purchasing.batchNumber")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("purchasing.expiryDate")}</Table.ColumnHeader>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {lines.map((l, idx) => {
                      const poItem = po.items.find((it) => it.id === l.purchaseOrderItemId);
                      return (
                        <Table.Row key={l.purchaseOrderItemId}>
                          <Table.Cell>
                            {poItem ? medicineName.get(poItem.medicineId) ?? poItem.medicineId.slice(0, 8) : "?"}
                            <Text fontSize="xs" color="fg.muted">
                              remaining: {l.remaining}
                            </Text>
                          </Table.Cell>
                          <Table.Cell>
                            <Input
                              size="sm"
                              type="number"
                              value={l.qty}
                              onChange={(e) =>
                                updateLine(idx, { qty: parseInt(e.target.value, 10) || 0 })
                              }
                              w="80px"
                              max={l.remaining}
                            />
                          </Table.Cell>
                          <Table.Cell>
                            <Input
                              size="sm"
                              type="number"
                              value={l.unitCostPrice}
                              onChange={(e) =>
                                updateLine(idx, {
                                  unitCostPrice: parseInt(e.target.value, 10) || 0,
                                })
                              }
                              w="120px"
                            />
                          </Table.Cell>
                          <Table.Cell>
                            <Input
                              size="sm"
                              value={l.batchNumber}
                              onChange={(e) => updateLine(idx, { batchNumber: e.target.value })}
                              w="120px"
                            />
                          </Table.Cell>
                          <Table.Cell>
                            <DatePickerField
                              size="sm"
                              value={l.expiryDate}
                              onChange={(v) => updateLine(idx, { expiryDate: v })}
                            />
                          </Table.Cell>
                        </Table.Row>
                      );
                    })}
                  </Table.Body>
                </Table.Root>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="full">
                <Button variant="ghost" onClick={onClose}>
                  {t("common.cancel")}
                </Button>
                <Button
                  colorPalette="blue"
                  onClick={submit}
                  loading={createReceipt.isPending}
                  disabled={!canSubmit}
                >
                  {t("purchasing.actions.receive")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

function PayDialog({
  open,
  onClose,
  poId,
  outstanding,
}: {
  open: boolean;
  onClose: () => void;
  poId: string;
  outstanding: number;
}) {
  const { t } = useTranslation();
  const payMut = usePayPurchaseMutation();
  const [amount, setAmount] = useState(String(outstanding));
  const [note, setNote] = useState("");

  const submit = async () => {
    const n = parseInt(amount, 10);
    if (!n || n <= 0) return;
    try {
      await payMut.mutateAsync({
        purchaseOrderId: poId,
        amount: BigInt(n),
        note,
      });
      toast.success(t("purchasing.actions.pay") + " ✓");
      onClose();
    } catch {
      /* */
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("purchasing.payTitle")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                <Box>
                  <Text fontSize="xs" color="fg.muted">
                    {t("purchasing.outstanding")}
                  </Text>
                  <Text fontSize="md" fontFamily="mono">
                    {formatMoney(outstanding)}
                  </Text>
                </Box>
                <Box>
                  <Text fontSize="xs" color="fg.muted" mb={1}>
                    {t("purchasing.amount")}
                  </Text>
                  <Input
                    type="number"
                    value={amount}
                    onChange={(e) => setAmount(e.target.value)}
                  />
                </Box>
                <Box>
                  <Text fontSize="xs" color="fg.muted" mb={1}>
                    {t("purchasing.note")}
                  </Text>
                  <Input value={note} onChange={(e) => setNote(e.target.value)} />
                </Box>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="full">
                <Button variant="ghost" onClick={onClose}>
                  {t("common.cancel")}
                </Button>
                <Button colorPalette="blue" onClick={submit} loading={payMut.isPending}>
                  {t("purchasing.actions.pay")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

function statusKey(s: POStatus): string {
  switch (s) {
    case POStatus.PO_STATUS_DRAFT:
      return "draft";
    case POStatus.PO_STATUS_SENT:
      return "sent";
    case POStatus.PO_STATUS_PARTIALLY_RECEIVED:
      return "partiallyReceived";
    case POStatus.PO_STATUS_RECEIVED:
      return "received";
    case POStatus.PO_STATUS_CLOSED:
      return "closed";
    case POStatus.PO_STATUS_VOIDED:
      return "voided";
    default:
      return "draft";
  }
}

