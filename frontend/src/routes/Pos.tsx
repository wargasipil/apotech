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
import { useQueryClient } from "@tanstack/react-query";
import { FileText, Lock, LogOut, Minus, Plus, Search, Trash2, UserRound, Warehouse as WarehouseIcon, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import EnumSelect from "../components/EnumSelect";
import MoneyInput from "../components/MoneyInput";
import WarehouseSelect from "../components/WarehouseSelect";
import { Medicine, type MedicineUnit } from "../gen/inventory_iface/v1/medicine_pb";
import { PaymentSource, Sale, SaleStatus, type SaleItem } from "../gen/pos_iface/v1/sale_pb";
import { Customer } from "../gen/customer_iface/v1/customer_pb";
import { saleClient } from "../lib/clients";
import { formatMoney } from "../lib/format";
import { toast } from "../lib/toaster";
import { useAuth } from "../lib/auth";
import { WAREHOUSE_KEY } from "../lib/transport";
import { useMyWarehousesQuery } from "../queries/warehouses";
import { useAllMedicinesQuery } from "../queries/medicines";
import { usePrescriptionsQuery } from "../queries/prescriptions";
import { ALL_LIMIT } from "../lib/pagination";
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
} from "../queries/sales";

export default function Pos() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
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

  const [sale, setSale] = useState<Sale | null>(null);
  const [completedSale, setCompletedSale] = useState<Sale | null>(null);
  const [customerOpen, setCustomerOpen] = useState(false);
  const [rxOpen, setRxOpen] = useState(false);
  // When the cashier clicks an Rx-required medicine without an attached Rx,
  // we open the picker filtered to that medicine and defer the AddItem call.
  // The AttachPrescription -> AddItem chain runs in `onAttachRx` once the
  // cashier picks an Rx. Reset to null whenever the picker closes.
  const [pendingAdd, setPendingAdd] = useState<{ medicineId: string; unitId: string } | null>(null);
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

  // Discard an abandoned cart when leaving POS. The active `sale` is always a
  // DRAFT (doComplete nulls it on completion), so deleting it on unmount cleans
  // up in-progress carts that never completed — they vanish entirely (no VOIDED
  // trace, never reach order history). Best-effort: raw client call with errors
  // swallowed (no global error toast). saleRef mirrors `sale` so the mount-once
  // cleanup reads the latest value.
  const saleRef = useRef<Sale | null>(null);
  useEffect(() => {
    saleRef.current = sale;
  }, [sale]);
  useEffect(() => {
    return () => {
      const s = saleRef.current;
      if (s && s.status === SaleStatus.DRAFT) {
        void saleClient.discardSale({ saleId: s.id }).catch(() => {});
      }
    };
  }, []);

  // --- Warehouse gate: pick the selling warehouse before POS opens ----
  // POS is full-screen (no TopBar selector), so the cashier chooses the active
  // warehouse here. Auto-skipped when they have <=1 warehouse or one is already
  // chosen. The choice drives the X-Warehouse-Id header (FEFO sells from this
  // warehouse only). "Change warehouse" clears the choice to re-open the gate.
  const myWarehousesQ = useMyWarehousesQuery();
  const [gateDone, setGateDone] = useState(false);
  const [currentWarehouse, setCurrentWarehouse] = useState<string>(
    () => localStorage.getItem(WAREHOUSE_KEY) ?? "",
  );
  const warehouses = myWarehousesQ.data?.warehouses ?? [];

  useEffect(() => {
    const data = myWarehousesQ.data;
    if (!data) return;
    if (data.warehouses.length === 0) {
      // No membership — proceed; the backend resolves the default warehouse.
      setGateDone(true);
      return;
    }
    const persisted = localStorage.getItem(WAREHOUSE_KEY);
    if (persisted && data.warehouses.some((w) => w.id === persisted)) {
      setCurrentWarehouse(persisted);
      setGateDone(true);
      return;
    }
    if (data.warehouses.length === 1) {
      localStorage.setItem(WAREHOUSE_KEY, data.warehouses[0].id);
      setCurrentWarehouse(data.warehouses[0].id);
      setGateDone(true);
    }
    // else: multiple warehouses + nothing chosen yet -> show the gate.
  }, [myWarehousesQ.data]);

  const confirmWarehouse = useCallback((id: string) => {
    const prev = localStorage.getItem(WAREHOUSE_KEY);
    localStorage.setItem(WAREHOUSE_KEY, id);
    setCurrentWarehouse(id);
    // Refetch warehouse-scoped data with the new header — no full reload.
    if (prev !== id) void queryClient.invalidateQueries();
    setGateDone(true);
  }, [queryClient]);

  const activeWarehouseName =
    warehouses.find((w) => w.id === currentWarehouse)?.name ?? "";

  // Switch the selling warehouse in place from the header picker. Stock is
  // per-warehouse, so the in-progress DRAFT cart is discarded (deleted, not
  // voided); the next add lazily starts a fresh draft stamped with the new
  // warehouse. Best-effort discard: raw client call, errors swallowed.
  const switchWarehouse = useCallback(
    async (id: string) => {
      if (id === currentWarehouse) return;
      if (sale) {
        await saleClient.discardSale({ saleId: sale.id }).catch(() => {});
        setSale(null);
      }
      localStorage.setItem(WAREHOUSE_KEY, id);
      setCurrentWarehouse(id);
      void queryClient.invalidateQueries();
    },
    [currentWarehouse, sale, queryClient],
  );

  useEffect(() => {
    void ensureSale();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Search ----
  const medicinesQ = useAllMedicinesQuery();
  const stockQ = useStockLevelsQuery();
  const stockByMedicine = useMemo(() => {
    const out = new Map<string, bigint>();
    for (const l of stockQ.data ?? []) {
      out.set(l.medicineId, (out.get(l.medicineId) ?? 0n) + l.currentQuantity);
    }
    return out;
  }, [stockQ.data]);
  const medById = useMemo(
    () => new Map(medicinesQ.rows.map((m) => [m.id, m])),
    [medicinesQ.rows],
  );

  const [query, setQuery] = useState("");
  const [highlight, setHighlight] = useState(0);
  const searchRef = useRef<HTMLInputElement | null>(null);

  // Each sellable unit of each matching medicine is its own search row, so one
  // click adds that exact unit. `available` = how many of that unit the current
  // base stock can make (base ÷ factor).
  type UnitRow = { med: Medicine; unit: MedicineUnit; available: number };
  const MAX_ROWS = 40;
  const unitRows = useMemo<UnitRow[]>(() => {
    const q = query.trim().toLowerCase();
    const meds = q
      ? medicinesQ.rows.filter((m) =>
          [m.sku, m.name].some((s) => s.toLowerCase().includes(q)),
        )
      : medicinesQ.rows;
    const out: UnitRow[] = [];
    for (const med of meds) {
      const base = Number(stockByMedicine.get(med.id) ?? 0n);
      for (const unit of med.units.filter((u) => u.sellable && u.active)) {
        const factor = Number(unit.factor) || 1;
        out.push({ med, unit, available: Math.floor(base / factor) });
        if (out.length >= MAX_ROWS) return out;
      }
    }
    return out;
  }, [query, medicinesQ.rows, stockByMedicine]);

  useEffect(() => {
    setHighlight(0);
  }, [query]);

  const onAdd = useCallback(
    async (medicine: Medicine, unitId: string, available?: number) => {
      const s = await ensureSale();
      if (!s) return;
      const enough =
        available !== undefined
          ? available >= 1
          : Number(stockByMedicine.get(medicine.id) ?? 0n) > 0;
      if (!enough) {
        toast.error(t("pos.outOfStock"));
        return;
      }
      // Rx-required medicines without an attached prescription: reroute to
      // the picker (filtered to this medicine) instead of letting the
      // backend reject the add. The deferred addItem (at the chosen unit)
      // happens in onAttachRx after the cashier picks an Rx.
      if (medicine.prescriptionRequired && !s.prescriptionId) {
        setPendingAdd({ medicineId: medicine.id, unitId });
        setRxOpen(true);
        return;
      }
      try {
        const res = await addItem.mutateAsync({
          saleId: s.id,
          medicineId: medicine.id,
          medicineUnitId: unitId,
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
      setHighlight((h) => Math.min(unitRows.length - 1, h + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => Math.max(0, h - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      // Barcode scanner: exact SKU match adds the medicine at its base unit.
      const skuExact = medicinesQ.rows.find(
        (m) => m.sku.toLowerCase() === query.trim().toLowerCase(),
      );
      if (skuExact) {
        const baseId = skuExact.units.find((u) => u.isBase)?.id ?? "";
        void onAdd(skuExact, baseId);
        return;
      }
      const row = unitRows[highlight];
      if (row) void onAdd(row.med, row.unit.id, row.available);
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

  // Switch a cart line's selling unit (box/strip/tablet): remove + re-add at the
  // new unit, keeping the same numeric qty.
  const onChangeUnit = async (item: SaleItem, unitId: string) => {
    if (!sale || unitId === item.medicineUnitId) return;
    try {
      await removeItem.mutateAsync({ saleId: sale.id, itemId: item.id });
      const res = await addItem.mutateAsync({
        saleId: sale.id,
        medicineId: item.medicineId,
        medicineUnitId: unitId,
        qty: item.qty,
      });
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
    const pending = pendingAdd;
    try {
      const attachRes = await attachRx.mutateAsync({
        saleId: sale.id,
        prescriptionId,
      });
      if (attachRes.sale) setSale(attachRes.sale);
      setRxOpen(false);
      // Pending-add chain: after a successful attach triggered by clicking
      // an Rx-required medicine, fire the deferred addItem at the chosen
      // unit. If addItem fails (e.g., remaining qty insufficient), the Rx
      // stays attached — the cashier sees the error toast and can retry.
      if (pending) {
        setPendingAdd(null);
        try {
          const addRes = await addItem.mutateAsync({
            saleId: sale.id,
            medicineId: pending.medicineId,
            medicineUnitId: pending.unitId,
            qty: 1,
          });
          if (addRes.sale) setSale(addRes.sale);
          setQuery("");
          searchRef.current?.focus();
        } catch {
          /* addItem error toast surfaces globally */
        }
      }
    } catch {
      /* attach error toast surfaces globally; pendingAdd left set so the
         cashier can retry by closing/reopening the picker */
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

  const onCloseReceipt = async () => {
    setCompletedSale(null);
    await ensureSale();
    searchRef.current?.focus();
  };

  // Warehouse gate: block POS until a selling warehouse is chosen.
  if (!gateDone) {
    return (
      <Flex direction="column" align="center" justify="center" h="100vh" bg="bg" gap={5} p={6}>
        <HStack gap={2} color="fg.muted">
          <WarehouseIcon size={20} />
          <Text fontSize="lg" fontWeight="semibold">
            {t("pos.selectWarehouse")}
          </Text>
        </HStack>
        <Text fontSize="sm" color="fg.muted">
          {t("pos.selectWarehouseHint")}
        </Text>
        <WarehouseSelect
          value=""
          onChange={confirmWarehouse}
          warehouses={warehouses}
          width="320px"
        />
        <Button variant="ghost" size="sm" onClick={() => navigate("/")}>
          <LogOut size={16} />
          {t("pos.exit")}
        </Button>
      </Flex>
    );
  }

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
          {warehouses.length > 1 ? (
            <WarehouseSelect
              value={currentWarehouse}
              onChange={switchWarehouse}
              warehouses={warehouses}
              size="xs"
              width="190px"
            />
          ) : activeWarehouseName ? (
            <HStack gap={1} color="fg.muted" px={2}>
              <WarehouseIcon size={14} />
              <Text fontSize="xs">{activeWarehouseName}</Text>
            </HStack>
          ) : null}
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
            {unitRows.map((row, i) => {
              const { med: m, unit, available } = row;
              const out = available < 1;
              // Rx-required medicine with no prescription attached yet: dim it
              // with a cue, but keep it clickable — the click auto-opens the
              // prescription picker (handled in onAdd).
              const needsRx = m.prescriptionRequired && !sale?.prescriptionId;
              const active = i === highlight;
              return (
                <Flex
                  key={`${m.id}:${unit.id}`}
                  px={3}
                  py={2}
                  borderRadius="md"
                  bg={active ? "bg.muted" : "transparent"}
                  borderWidth="1px"
                  borderColor={active ? "border" : "transparent"}
                  align="center"
                  justify="space-between"
                  cursor={out ? "not-allowed" : "pointer"}
                  opacity={out ? 0.5 : needsRx ? 0.65 : 1}
                  onMouseEnter={() => setHighlight(i)}
                  onClick={() => !out && onAdd(m, unit.id, available)}
                >
                  <Stack gap={0} flex="1">
                    <HStack gap={2}>
                      <Text fontSize="sm" fontWeight="medium">
                        {m.name}
                      </Text>
                      <Text fontSize="xs" color="fg.muted">
                        · {unit.name}
                      </Text>
                      {m.prescriptionRequired && (
                        <Badge size="xs" colorPalette="red">
                          {t("inventory.medicines.rxShort")}
                        </Badge>
                      )}
                      {needsRx && (
                        <HStack gap={1} color="orange.fg">
                          <Lock size={12} />
                          <Text fontSize="xs" fontWeight="medium">
                            {t("pos.needsRx")}
                          </Text>
                        </HStack>
                      )}
                    </HStack>
                    <Text fontSize="xs" color="fg.muted">
                      {m.sku} · {available} {unit.name}
                    </Text>
                  </Stack>
                  <Text fontSize="sm" fontFamily="mono">
                    {formatMoney(unit.sellPrice)}
                  </Text>
                </Flex>
              );
            })}
            {unitRows.length === 0 && (
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
                const med = medicinesQ.rows.find((m) => m.id === it.medicineId);
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
                    <IconButton
                      aria-label="decrease quantity"
                      size="xs"
                      variant="outline"
                      onClick={() => onChangeQty(it.id, it.qty - 1)}
                    >
                      <Minus size={14} />
                    </IconButton>
                    <Input
                      size="sm"
                      type="number"
                      value={it.qty}
                      onChange={(e) => onChangeQty(it.id, parseInt(e.target.value, 10) || 0)}
                      w="48px"
                      textAlign="center"
                    />
                    <IconButton
                      aria-label="increase quantity"
                      size="xs"
                      variant="outline"
                      onClick={() => onChangeQty(it.id, it.qty + 1)}
                    >
                      <Plus size={14} />
                    </IconButton>
                    {med && med.units.filter((u) => u.sellable && u.active).length > 1 ? (
                      <EnumSelect
                        size="sm"
                        width="84px"
                        value={it.medicineUnitId}
                        onChange={(v) => onChangeUnit(it, v)}
                        items={med.units.filter((u) => u.sellable && u.active)}
                        itemToString={(u) => u.name}
                        itemToValue={(u) => u.id}
                      />
                    ) : (
                      <Text fontSize="xs" color="fg.muted" w="84px">
                        {it.unitName || med?.unit}
                      </Text>
                    )}
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
                    <MoneyInput
                      size="sm"
                      value={paidAmount}
                      onChange={setPaidAmount}
                    />
                  </HStack>
                  <QuickAmountRow
                    total={total}
                    onPick={(n) => setPaidAmount(String(n))}
                  />
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
                  colorPalette="blue"
                  flex="1"
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
        requiredMedicineId={pendingAdd?.medicineId ?? null}
        medById={medById}
        onClose={() => {
          setRxOpen(false);
          setPendingAdd(null);
        }}
        onPick={onAttachRx}
      />

      <ReceiptDialog sale={completedSale} onClose={onCloseReceipt} />
    </Flex>
  );
}

// QuickAmountRow: one-tap fill of the paid input. Renders below the Dibayar
// field for Cash payments. Includes an "Exact" chip (paid = total), an
// optional round-up-to-next-10k chip, and standard IDR banknote denominations
// (5k/10k/20k/50k/100k) filtered to amounts >= total.
function QuickAmountRow({
  total,
  onPick,
}: {
  total: number;
  onPick: (n: number) => void;
}) {
  const { t } = useTranslation();
  if (total <= 0) return null;
  const DENOMS = [5_000, 10_000, 20_000, 50_000, 100_000];
  const above = DENOMS.filter((d) => d >= total);
  const roundedUp = Math.ceil(total / 10_000) * 10_000;
  const showRoundUp = roundedUp !== total && !above.includes(roundedUp);
  return (
    <Flex wrap="wrap" gap={1} mt={1}>
      <Button
        size="xs"
        variant="outline"
        colorPalette="blue"
        onClick={() => onPick(total)}
      >
        {t("pos.exactAmount")}
      </Button>
      {showRoundUp && (
        <Button
          size="xs"
          variant="outline"
          colorPalette="blue"
          onClick={() => onPick(roundedUp)}
        >
          {formatMoney(roundedUp)}
        </Button>
      )}
      {above.map((d) => (
        <Button
          key={d}
          size="xs"
          variant="outline"
          colorPalette="blue"
          onClick={() => onPick(d)}
        >
          {formatMoney(d)}
        </Button>
      ))}
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
  requiredMedicineId,
  medById,
  onClose,
  onPick,
}: {
  open: boolean;
  customerId: string;
  /** When set, the picker is in "pending-add" mode: filter to Rx that
   * contain this medicine with remaining qty > 0, and surface a caption
   * + empty-state link explaining the context. */
  requiredMedicineId: string | null;
  /** Page-level medicine lookup for displaying per-item names. */
  medById: Map<string, Medicine>;
  onClose: () => void;
  onPick: (prescriptionId: string) => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  // List ACTIVE Rx; filter to current customer when one is set. Load the full
  // active set (not just a page) so the picker shows every covering Rx.
  const rxQ = usePrescriptionsQuery({ status: "ACTIVE", customerId, limit: ALL_LIMIT });

  // In pending-add mode, narrow the loaded prescriptions to ones that
  // include the required medicine with remaining qty. Done client-side on
  // the page-of-active-Rx the query already returns — no extra RPC.
  const visibleRxs = useMemo(() => {
    const all = rxQ.rows;
    if (!requiredMedicineId) return all;
    return all.filter((rx) =>
      rx.items.some(
        (it) => it.medicineId === requiredMedicineId && it.dispensedQty < it.prescribedQty,
      ),
    );
  }, [rxQ.rows, requiredMedicineId]);

  const requiredMedicineName =
    requiredMedicineId ? medById.get(requiredMedicineId)?.name ?? requiredMedicineId : "";

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
              {requiredMedicineId && (
                <Box
                  mb={3}
                  px={3}
                  py={2}
                  borderRadius="md"
                  bg="orange.subtle"
                  borderWidth="1px"
                  borderColor="orange.muted"
                >
                  <Text fontSize="sm">
                    {t("prescriptions.needForMedicine", { medicine: requiredMedicineName })}
                  </Text>
                </Box>
              )}
              <Stack gap={2} maxH="420px" overflowY="auto">
                {visibleRxs.map((rx) => (
                  <Box
                    key={rx.id}
                    borderWidth="1px"
                    borderRadius="md"
                    _hover={{ bg: "bg.muted" }}
                    cursor="pointer"
                    onClick={() => onPick(rx.id)}
                  >
                    <Flex px={3} py={2} justify="space-between" align="center">
                      <Stack gap={0}>
                        <HStack gap={2}>
                          <Text fontFamily="mono" fontSize="sm">
                            {rx.rxNo}
                          </Text>
                          <Badge size="xs" colorPalette="green">
                            {t("prescriptions.states.active")}
                          </Badge>
                        </HStack>
                        <Text fontSize="xs" color="fg.muted">
                          {rx.issuerName} · {rx.issuedAt} → {rx.expiresAt}
                        </Text>
                      </Stack>
                      <Plus size={14} />
                    </Flex>
                    <Stack gap={0} borderTopWidth="1px" px={3} py={2} bg="bg.subtle">
                      {rx.items.map((it) => {
                        const remaining = it.prescribedQty - it.dispensedQty;
                        const name = medById.get(it.medicineId)?.name ?? it.medicineId.slice(0, 8);
                        const exhausted = remaining <= 0;
                        const isMatch = it.medicineId === requiredMedicineId;
                        return (
                          <Flex
                            key={it.id}
                            justify="space-between"
                            fontSize="xs"
                            color={exhausted ? "fg.subtle" : isMatch ? "green.fg" : "fg"}
                            textDecoration={exhausted ? "line-through" : undefined}
                            py={0.5}
                          >
                            <Text>{name}</Text>
                            <Text fontFamily="mono">
                              {remaining}/{it.prescribedQty} {t("prescriptions.remaining")}
                            </Text>
                          </Flex>
                        );
                      })}
                    </Stack>
                  </Box>
                ))}
                {visibleRxs.length === 0 && (
                  <Stack gap={2} textAlign="center" py={6}>
                    <Text color="fg.muted" fontSize="sm">
                      {requiredMedicineId
                        ? t("prescriptions.rxRequiredMissing")
                        : t("common.noResults")}
                    </Text>
                    {requiredMedicineId && (
                      <Button
                        size="xs"
                        variant="outline"
                        alignSelf="center"
                        onClick={() => {
                          onClose();
                          navigate("/prescriptions");
                        }}
                      >
                        {t("prescriptions.goCreate")}
                      </Button>
                    )}
                  </Stack>
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
  const medicinesQ = useAllMedicinesQuery();
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
  const medById = new Map(medicinesQ.rows.map((m) => [m.id, m]));
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
                        {it.qty}
                        {it.unitName ? ` ${it.unitName}` : ""}×{" "}
                        {medById.get(it.medicineId)?.name ?? it.medicineId.slice(0, 8)}
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
