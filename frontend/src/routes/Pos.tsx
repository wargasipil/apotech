import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import {
  Badge,
  Box,
  Button,
  Dialog,
  Flex,
  HStack,
  IconButton,
  Input,
  Portal,
  RadioGroup,
  Stack,
  Text,
} from "@chakra-ui/react";
import { FileText, LogOut, Plus, Search, Trash2, UserRound, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { Medicine } from "../gen/inventory_iface/v1/medicine_pb";
import { PaymentSource, Sale } from "../gen/pos_iface/v1/sale_pb";
import { Customer } from "../gen/customer_iface/v1/customer_pb";
import { formatMoney } from "../lib/format";
import { toast } from "../lib/toaster";
import { useAuth } from "../lib/auth";
import { useMedicinesQuery } from "../queries/medicines";
import { usePrescriptionsQuery } from "../queries/prescriptions";
import { useStockLevelsQuery } from "../queries/stock";
import { useCustomerSearchQuery } from "../queries/customers";
import {
  useAddItemMutation,
  useAttachPrescriptionMutation,
  useCompleteSaleMutation,
  useDetachPrescriptionMutation,
  usePrintReceiptMutation,
  useRemoveItemMutation,
  useSetItemQuantityMutation,
  useSetSaleCustomerMutation,
  useStartSaleMutation,
  useVoidSaleMutation,
} from "../queries/sales";

export default function Pos() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { user } = useAuth();

  // Sale lifecycle ----
  const startSale = useStartSaleMutation();
  const addItem = useAddItemMutation();
  const setQty = useSetItemQuantityMutation();
  const removeItem = useRemoveItemMutation();
  const setSaleCustomer = useSetSaleCustomerMutation();
  const attachRx = useAttachPrescriptionMutation();
  const detachRx = useDetachPrescriptionMutation();
  const completeSale = useCompleteSaleMutation();
  const voidSale = useVoidSaleMutation();

  const [sale, setSale] = useState<Sale | null>(null);
  const [completedSale, setCompletedSale] = useState<Sale | null>(null);
  const [customerOpen, setCustomerOpen] = useState(false);
  const [rxOpen, setRxOpen] = useState(false);
  const [paymentSource, setPaymentSource] = useState<PaymentSource>(
    PaymentSource.CASH,
  );
  const [paidAmount, setPaidAmount] = useState("0");

  const ensureSale = useCallback(async (): Promise<Sale | null> => {
    if (sale) return sale;
    try {
      const res = await startSale.mutateAsync();
      if (res.sale) setSale(res.sale);
      return res.sale ?? null;
    } catch {
      return null;
    }
  }, [sale, startSale]);

  useEffect(() => {
    void ensureSale();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Search ----
  const medicinesQ = useMedicinesQuery();
  const stockQ = useStockLevelsQuery();
  const stockByMedicine = useMemo(() => {
    const out = new Map<string, bigint>();
    for (const l of stockQ.data ?? []) {
      out.set(l.medicineId, (out.get(l.medicineId) ?? 0n) + l.currentQuantity);
    }
    return out;
  }, [stockQ.data]);

  const [query, setQuery] = useState("");
  const [highlight, setHighlight] = useState(0);
  const searchRef = useRef<HTMLInputElement | null>(null);

  const filtered = useMemo(() => {
    const all = medicinesQ.data ?? [];
    if (!query.trim()) return all.slice(0, 30);
    const q = query.toLowerCase();
    return all
      .filter((m) =>
        [m.sku, m.name, m.manufacturer].some((s) => s.toLowerCase().includes(q)),
      )
      .slice(0, 30);
  }, [query, medicinesQ.data]);

  useEffect(() => {
    setHighlight(0);
  }, [query]);

  const onAdd = useCallback(
    async (medicine: Medicine) => {
      const s = await ensureSale();
      if (!s) return;
      const stock = Number(stockByMedicine.get(medicine.id) ?? 0n);
      if (stock <= 0) {
        toast.error(t("pos.outOfStock"));
        return;
      }
      try {
        const res = await addItem.mutateAsync({
          saleId: s.id,
          medicineId: medicine.id,
          qty: 1,
        });
        if (res.sale) setSale(res.sale);
        setQuery("");
        searchRef.current?.focus();
      } catch {
        /* toast handled globally */
      }
    },
    [ensureSale, addItem, stockByMedicine, t],
  );

  const onSearchKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlight((h) => Math.min(filtered.length - 1, h + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => Math.max(0, h - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      // Barcode scanner: if exact SKU match, add it directly.
      const skuExact = (medicinesQ.data ?? []).find(
        (m) => m.sku.toLowerCase() === query.trim().toLowerCase(),
      );
      const target = skuExact ?? filtered[highlight];
      if (target) void onAdd(target);
    } else if (e.key === "Escape") {
      setQuery("");
    }
  };

  // Global keyboard shortcuts ----
  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key === "F2") {
        e.preventDefault();
        searchRef.current?.focus();
      } else if (e.key === "F4") {
        e.preventDefault();
        setCustomerOpen(true);
      } else if (e.key === "F5") {
        e.preventDefault();
        setRxOpen(true);
      } else if (e.key === "F8") {
        e.preventDefault();
        void doComplete();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sale, paymentSource, paidAmount]);

  // Cart ops ----
  const onChangeQty = async (itemId: string, qty: number) => {
    if (!sale) return;
    if (qty <= 0) {
      try {
        const res = await removeItem.mutateAsync({ saleId: sale.id, itemId });
        if (res.sale) setSale(res.sale);
      } catch {
        /* toast handled globally */
      }
      return;
    }
    try {
      const res = await setQty.mutateAsync({ saleId: sale.id, itemId, qty });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally */
    }
  };

  const onRemove = async (itemId: string) => {
    if (!sale) return;
    try {
      const res = await removeItem.mutateAsync({ saleId: sale.id, itemId });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally */
    }
  };

  const onAttachCustomer = async (customerId: string) => {
    if (!sale) return;
    try {
      const res = await setSaleCustomer.mutateAsync({
        saleId: sale.id,
        customerId,
      });
      if (res.sale) setSale(res.sale);
      setCustomerOpen(false);
    } catch {
      /* toast handled globally */
    }
  };

  const onClearCustomer = async () => {
    if (!sale) return;
    try {
      const res = await setSaleCustomer.mutateAsync({
        saleId: sale.id,
        customerId: "",
      });
      if (res.sale) setSale(res.sale);
    } catch {
      /* */
    }
  };

  const onAttachRx = async (prescriptionId: string) => {
    if (!sale) return;
    try {
      const res = await attachRx.mutateAsync({ saleId: sale.id, prescriptionId });
      if (res.sale) setSale(res.sale);
      setRxOpen(false);
    } catch {
      /* toast handled globally */
    }
  };

  const onDetachRx = async () => {
    if (!sale) return;
    try {
      const res = await detachRx.mutateAsync({ saleId: sale.id });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally */
    }
  };

  const total = Number(sale?.total ?? 0n);
  const paidNum = Number(paidAmount || "0") || 0;
  const change = paidNum - total;
  const canComplete =
    !!sale &&
    sale.items.length > 0 &&
    (paymentSource !== PaymentSource.CASH || paidNum >= total);

  const doComplete = useCallback(async () => {
    if (!canComplete || !sale) return;
    try {
      const res = await completeSale.mutateAsync({
        saleId: sale.id,
        paymentSource,
        paidAmount: BigInt(paidNum),
      });
      if (res.sale) setCompletedSale(res.sale);
      setSale(null);
      setQuery("");
      setPaidAmount("0");
      setPaymentSource(PaymentSource.CASH);
    } catch {
      /* toast handled globally */
    }
  }, [canComplete, sale, completeSale, paymentSource, paidNum]);

  const onCancelSale = async () => {
    if (!sale) return;
    try {
      await voidSale.mutateAsync({ saleId: sale.id });
      setSale(null);
      setQuery("");
    } catch {
      /* */
    }
  };

  const onCloseReceipt = async () => {
    setCompletedSale(null);
    await ensureSale();
    searchRef.current?.focus();
  };

  return (
    <Flex direction="column" h="100vh" bg="bg">
      {/* Header strip */}
      <Flex
        align="center"
        justify="space-between"
        px={4}
        h="48px"
        borderBottomWidth="1px"
      >
        <Text fontWeight="semibold">{t("pos.title")}</Text>
        <HStack gap={2}>
          {user && (
            <Text fontSize="sm" color="fg.muted">
              {user.name || user.email}
            </Text>
          )}
          <IconButton
            aria-label="exit"
            variant="ghost"
            size="sm"
            onClick={() => navigate("/")}
          >
            <LogOut size={16} />
          </IconButton>
        </HStack>
      </Flex>

      {/* Body */}
      <Flex flex="1" minH={0}>
        {/* Search panel */}
        <Box flex="3" borderRightWidth="1px" overflowY="auto" p={4}>
          <Box position="relative" mb={3}>
            <Box position="absolute" left={3} top="50%" transform="translateY(-50%)" color="fg.muted">
              <Search size={16} />
            </Box>
            <Input
              ref={searchRef}
              pl={10}
              placeholder={t("pos.searchPlaceholder")}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={onSearchKeyDown}
              autoFocus
            />
          </Box>
          <Stack gap={1}>
            {filtered.map((m, i) => {
              const stock = Number(stockByMedicine.get(m.id) ?? 0n);
              const out = stock <= 0;
              const active = i === highlight;
              return (
                <Flex
                  key={m.id}
                  px={3}
                  py={2}
                  borderRadius="md"
                  bg={active ? "bg.muted" : "transparent"}
                  borderWidth="1px"
                  borderColor={active ? "border" : "transparent"}
                  align="center"
                  justify="space-between"
                  cursor={out ? "not-allowed" : "pointer"}
                  opacity={out ? 0.5 : 1}
                  onMouseEnter={() => setHighlight(i)}
                  onClick={() => !out && onAdd(m)}
                >
                  <Stack gap={0} flex="1">
                    <HStack gap={2}>
                      <Text fontSize="sm" fontWeight="medium">
                        {m.name}
                      </Text>
                      {m.prescriptionRequired && (
                        <Badge size="xs" colorPalette="red">
                          {t("inventory.medicines.rxShort")}
                        </Badge>
                      )}
                    </HStack>
                    <Text fontSize="xs" color="fg.muted">
                      {m.sku} · {stock} {t("pos.stock")}
                    </Text>
                  </Stack>
                  <Text fontSize="sm" fontFamily="mono">
                    {formatMoney(m.unitPrice)}
                  </Text>
                </Flex>
              );
            })}
            {filtered.length === 0 && (
              <Text color="fg.muted" fontSize="sm" textAlign="center" py={6}>
                {t("common.noResults")}
              </Text>
            )}
          </Stack>
        </Box>

        {/* Cart panel */}
        <Flex flex="2" direction="column" minW="360px">
          <Box px={4} py={3} borderBottomWidth="1px">
            <Flex justify="space-between" align="center">
              <Text fontWeight="semibold">{t("pos.cart")}</Text>
              <Text fontSize="sm" color="fg.muted">
                {sale?.items.length ?? 0} {t("pos.items")}
              </Text>
            </Flex>
            <CustomerBar
              sale={sale}
              onAttach={() => setCustomerOpen(true)}
              onClear={onClearCustomer}
            />
            <PrescriptionBar
              sale={sale}
              onAttach={() => setRxOpen(true)}
              onDetach={onDetachRx}
            />
          </Box>

          <Box flex="1" overflowY="auto" px={4} py={2}>
            {(sale?.items.length ?? 0) === 0 && (
              <Text color="fg.muted" fontSize="sm" textAlign="center" py={8}>
                {t("pos.empty")}
              </Text>
            )}
            <Stack gap={2}>
              {sale?.items.map((it) => {
                const med = medicinesQ.data?.find((m) => m.id === it.medicineId);
                return (
                  <Flex key={it.id} align="center" gap={2}>
                    <Stack gap={0} flex="1">
                      <Text fontSize="sm" fontWeight="medium">
                        {med?.name ?? it.medicineId.slice(0, 8)}
                      </Text>
                      <Text fontSize="xs" color="fg.muted" fontFamily="mono">
                        {formatMoney(it.unitPriceSnapshot)}
                      </Text>
                    </Stack>
                    <Input
                      size="sm"
                      type="number"
                      value={it.qty}
                      onChange={(e) => onChangeQty(it.id, parseInt(e.target.value, 10) || 0)}
                      w="64px"
                      textAlign="right"
                    />
                    <Text fontSize="sm" fontFamily="mono" w="80px" textAlign="right">
                      {formatMoney(it.lineTotal)}
                    </Text>
                    <IconButton
                      aria-label="remove"
                      size="xs"
                      variant="ghost"
                      onClick={() => onRemove(it.id)}
                    >
                      <Trash2 size={14} />
                    </IconButton>
                  </Flex>
                );
              })}
            </Stack>
          </Box>

          {/* Totals + payment */}
          <Box borderTopWidth="1px" px={4} py={3}>
            <Stack gap={2}>
              <Flex justify="space-between">
                <Text fontSize="sm" color="fg.muted">{t("pos.subtotal")}</Text>
                <Text fontSize="sm" fontFamily="mono">
                  {formatMoney(Number(sale?.subtotal ?? 0n))}
                </Text>
              </Flex>
              <Flex justify="space-between">
                <Text fontWeight="semibold">{t("pos.total")}</Text>
                <Text fontWeight="semibold" fontFamily="mono">
                  {formatMoney(total)}
                </Text>
              </Flex>

              <Box pt={2}>
                <Text fontSize="xs" color="fg.muted" mb={1}>
                  {t("pos.payment")}
                </Text>
                <RadioGroup.Root
                  value={String(paymentSource)}
                  onValueChange={(d) => setPaymentSource(Number(d.value) as PaymentSource)}
                >
                  <HStack gap={3}>
                    <RadioGroup.Item value={String(PaymentSource.CASH)}>
                      <RadioGroup.ItemHiddenInput />
                      <RadioGroup.ItemIndicator />
                      <RadioGroup.ItemText>{t("pos.paymentCash")}</RadioGroup.ItemText>
                    </RadioGroup.Item>
                    <RadioGroup.Item value={String(PaymentSource.BPJS)}>
                      <RadioGroup.ItemHiddenInput />
                      <RadioGroup.ItemIndicator />
                      <RadioGroup.ItemText>{t("pos.paymentBpjs")}</RadioGroup.ItemText>
                    </RadioGroup.Item>
                    <RadioGroup.Item value={String(PaymentSource.INSURANCE_OTHER)}>
                      <RadioGroup.ItemHiddenInput />
                      <RadioGroup.ItemIndicator />
                      <RadioGroup.ItemText>{t("pos.paymentOther")}</RadioGroup.ItemText>
                    </RadioGroup.Item>
                  </HStack>
                </RadioGroup.Root>
              </Box>

              {paymentSource === PaymentSource.CASH && (
                <Stack gap={1}>
                  <HStack>
                    <Text fontSize="xs" color="fg.muted" minW="56px">{t("pos.paid")}</Text>
                    <Input
                      size="sm"
                      type="number"
                      inputMode="numeric"
                      value={paidAmount}
                      onChange={(e) => setPaidAmount(e.target.value)}
                    />
                  </HStack>
                  <Flex justify="space-between">
                    <Text fontSize="xs" color="fg.muted">{t("pos.change")}</Text>
                    <Text fontSize="sm" fontFamily="mono" color={change < 0 ? "fg.error" : "fg"}>
                      {formatMoney(Math.max(0, change))}
                    </Text>
                  </Flex>
                </Stack>
              )}

              <HStack gap={2} pt={2}>
                <Button
                  variant="outline"
                  flex="1"
                  onClick={onCancelSale}
                  disabled={!sale}
                >
                  {t("pos.cancel")}
                </Button>
                <Button
                  colorPalette="blue"
                  flex="2"
                  onClick={doComplete}
                  disabled={!canComplete}
                  loading={completeSale.isPending}
                >
                  {t("pos.complete")}
                </Button>
              </HStack>
            </Stack>
          </Box>

          <Box bg="bg.muted" px={4} py={2} borderTopWidth="1px">
            <Text fontSize="xs" color="fg.muted">
              {t("pos.shortcutHints")}
            </Text>
          </Box>
        </Flex>
      </Flex>

      <CustomerPickerDialog
        open={customerOpen}
        onClose={() => setCustomerOpen(false)}
        onPick={onAttachCustomer}
      />

      <PrescriptionPickerDialog
        open={rxOpen}
        customerId={sale?.customerId ?? ""}
        onClose={() => setRxOpen(false)}
        onPick={onAttachRx}
      />

      <ReceiptDialog sale={completedSale} onClose={onCloseReceipt} />
    </Flex>
  );
}

function CustomerBar({
  sale,
  onAttach,
  onClear,
}: {
  sale: Sale | null;
  onAttach: () => void;
  onClear: () => void;
}) {
  const { t } = useTranslation();
  const hasCustomer = !!sale?.customerId;
  return (
    <Flex mt={2} align="center" gap={2}>
      <UserRound size={14} />
      <Text fontSize="xs" color="fg.muted" flex="1">
        {hasCustomer ? sale!.customerId.slice(0, 8) : t("pos.customer")}
      </Text>
      {hasCustomer ? (
        <Button size="xs" variant="ghost" onClick={onClear}>
          {t("pos.clearCustomer")}
        </Button>
      ) : (
        <Button size="xs" variant="ghost" onClick={onAttach}>
          {t("pos.attachCustomer")}
        </Button>
      )}
    </Flex>
  );
}

function PrescriptionBar({
  sale,
  onAttach,
  onDetach,
}: {
  sale: Sale | null;
  onAttach: () => void;
  onDetach: () => void;
}) {
  const { t } = useTranslation();
  const hasRx = !!sale?.prescriptionId;
  return (
    <Flex mt={1} align="center" gap={2}>
      <FileText size={14} />
      <Text fontSize="xs" color="fg.muted" flex="1">
        {hasRx ? t("prescriptions.attached") : t("prescriptions.title")}
      </Text>
      {hasRx ? (
        <Button size="xs" variant="ghost" onClick={onDetach}>
          {t("prescriptions.detach")}
        </Button>
      ) : (
        <Button size="xs" variant="ghost" onClick={onAttach}>
          {t("prescriptions.attach")}
        </Button>
      )}
    </Flex>
  );
}

function PrescriptionPickerDialog({
  open,
  customerId,
  onClose,
  onPick,
}: {
  open: boolean;
  customerId: string;
  onClose: () => void;
  onPick: (prescriptionId: string) => void;
}) {
  const { t } = useTranslation();
  // List ACTIVE Rx; filter to current customer when one is set.
  const rxQ = usePrescriptionsQuery({ status: "ACTIVE", customerId });

  if (!open) return null;
  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("prescriptions.attach")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={1} maxH="400px" overflowY="auto">
                {(rxQ.data ?? []).map((rx) => (
                  <Flex
                    key={rx.id}
                    px={3}
                    py={2}
                    borderRadius="md"
                    borderWidth="1px"
                    _hover={{ bg: "bg.muted" }}
                    cursor="pointer"
                    justify="space-between"
                    align="center"
                    onClick={() => onPick(rx.id)}
                  >
                    <Stack gap={0}>
                      <HStack gap={2}>
                        <Text fontFamily="mono" fontSize="sm">{rx.rxNo}</Text>
                        <Badge size="xs" colorPalette="green">
                          {t("prescriptions.states.active")}
                        </Badge>
                      </HStack>
                      <Text fontSize="xs" color="fg.muted">
                        {rx.issuerName} · {rx.issuedAt} → {rx.expiresAt} · {rx.items.length}{" "}
                        {t("prescriptions.items").toLowerCase()}
                      </Text>
                    </Stack>
                    <Plus size={14} />
                  </Flex>
                ))}
                {(rxQ.data?.length ?? 0) === 0 && (
                  <Text color="fg.muted" fontSize="sm" textAlign="center" py={6}>
                    {t("common.noResults")}
                  </Text>
                )}
              </Stack>
            </Dialog.Body>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

function CustomerPickerDialog({
  open,
  onClose,
  onPick,
}: {
  open: boolean;
  onClose: () => void;
  onPick: (customerId: string) => void;
}) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const searchQ = useCustomerSearchQuery(q, open);

  if (!open) return null;
  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("pos.attachCustomer")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                <Input
                  placeholder={t("customers.searchPlaceholder")}
                  value={q}
                  onChange={(e) => setQ(e.target.value)}
                  autoFocus
                />
                <Stack gap={1} maxH="320px" overflowY="auto">
                  {(searchQ.data ?? []).map((c: Customer) => (
                    <Flex
                      key={c.id}
                      px={3}
                      py={2}
                      borderRadius="md"
                      _hover={{ bg: "bg.muted" }}
                      cursor="pointer"
                      justify="space-between"
                      onClick={() => onPick(c.id)}
                    >
                      <Stack gap={0}>
                        <Text fontSize="sm" fontWeight="medium">{c.name}</Text>
                        <Text fontSize="xs" color="fg.muted">
                          {c.phone || c.bpjsNo || "—"}
                        </Text>
                      </Stack>
                      <Plus size={14} />
                    </Flex>
                  ))}
                  {(searchQ.data?.length ?? 0) === 0 && (
                    <Text color="fg.muted" fontSize="sm" textAlign="center" py={4}>
                      {t("common.noResults")}
                    </Text>
                  )}
                </Stack>
              </Stack>
            </Dialog.Body>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

function ReceiptDialog({ sale, onClose }: { sale: Sale | null; onClose: () => void }) {
  const { t } = useTranslation();
  const { data: medicines } = useMedicinesQuery();
  const printMut = usePrintReceiptMutation();
  if (!sale) return null;
  const onPrint = async () => {
    try {
      await printMut.mutateAsync(sale.id);
      toast.success(t("pos.printSent"));
    } catch {
      /* toast handled globally */
    }
  };
  const medById = new Map((medicines ?? []).map((m) => [m.id, m]));
  return (
    <Dialog.Root open={!!sale} onOpenChange={(d) => !d.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>
                {t("pos.receiptTitle")} · {sale.saleNo || sale.id.slice(0, 8)}
              </Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3} fontFamily="mono">
                <Stack gap={1}>
                  {sale.items.map((it) => (
                    <Flex key={it.id} justify="space-between" gap={2}>
                      <Text fontSize="sm" flex="1">
                        {it.qty}× {medById.get(it.medicineId)?.name ?? it.medicineId.slice(0, 8)}
                      </Text>
                      <Text fontSize="sm">{formatMoney(it.lineTotal)}</Text>
                    </Flex>
                  ))}
                </Stack>
                <Box borderTopWidth="1px" pt={2}>
                  <Flex justify="space-between">
                    <Text fontSize="sm">{t("pos.subtotal")}</Text>
                    <Text fontSize="sm">{formatMoney(Number(sale.subtotal))}</Text>
                  </Flex>
                  <Flex justify="space-between">
                    <Text fontWeight="semibold">{t("pos.total")}</Text>
                    <Text fontWeight="semibold">{formatMoney(Number(sale.total))}</Text>
                  </Flex>
                  <Flex justify="space-between">
                    <Text fontSize="sm" color="fg.muted">{t("pos.paid")}</Text>
                    <Text fontSize="sm">{formatMoney(Number(sale.paidAmount))}</Text>
                  </Flex>
                  <Flex justify="space-between">
                    <Text fontSize="sm" color="fg.muted">{t("pos.change")}</Text>
                    <Text fontSize="sm">
                      {formatMoney(Math.max(0, Number(sale.paidAmount) - Number(sale.total)))}
                    </Text>
                  </Flex>
                </Box>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <Button variant="outline" onClick={onPrint} loading={printMut.isPending}>
                {t("pos.print")}
              </Button>
              <Button colorPalette="blue" onClick={onClose}>
                {t("pos.newSale")}
              </Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
