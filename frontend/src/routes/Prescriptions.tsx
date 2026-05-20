import {
  Badge,
  Box,
  Button,
  Flex,
  HStack,
  Heading,
  IconButton,
  Input,
  Spinner,
  Stack,
  Table,
  Text,
  Textarea,
} from "@chakra-ui/react";
import { Pencil, Plus, Trash2, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import EntityDrawer from "../components/EntityDrawer";
import EnumSelect from "../components/EnumSelect";
import PageHeader from "../components/PageHeader";
import SearchableSelect from "../components/SearchableSelect";
import type { Prescription } from "../gen/prescription_iface/v1/prescription_pb";
import { formatDate } from "../lib/format";
import { toast } from "../lib/toaster";
import { searchCustomers, useCustomersQuery } from "../queries/customers";
import { searchMedicines, useMedicinesQuery } from "../queries/medicines";
import {
  useCreatePrescriptionMutation,
  usePrescriptionsQuery,
  useUpdatePrescriptionMutation,
  useVoidPrescriptionMutation,
} from "../queries/prescriptions";

const ALL_STATUSES = ["ACTIVE", "DISPENSED", "EXPIRED", "VOIDED"] as const;

type Line = {
  medicineId: string;
  prescribedQty: number;
  dosageInstructions: string;
  note: string;
};

const emptyLine: Line = { medicineId: "", prescribedQty: 1, dosageInstructions: "", note: "" };

function statusBadge(status: string) {
  const palette =
    status === "ACTIVE"
      ? "green"
      : status === "DISPENSED"
        ? "blue"
        : status === "EXPIRED"
          ? "red"
          : "gray";
  return palette;
}

function isoToday() {
  return new Date().toISOString().slice(0, 10);
}

export default function Prescriptions() {
  const { t } = useTranslation();
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [customerFilter, setCustomerFilter] = useState<string>("");
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Prescription | null>(null);

  const customersQ = useCustomersQuery(false);
  const rxQ = usePrescriptionsQuery({
    status: statusFilter,
    customerId: customerFilter,
  });
  const voidMut = useVoidPrescriptionMutation();

  const customerNameById = useMemo(() => {
    const m = new Map<string, string>();
    (customersQ.data ?? []).forEach((c) => m.set(c.id, c.name));
    return m;
  }, [customersQ.data]);

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("prescriptions.title") }]}
        title={t("prescriptions.title")}
        actions={
          <Button colorPalette="blue" onClick={() => setCreateOpen(true)}>
            <Plus size={16} />
            {t("prescriptions.addTitle")}
          </Button>
        }
      />

      <Flex gap={3} mb={4} wrap="wrap">
        <Box minW="180px">
          <Text fontSize="xs" color="fg.muted" mb={1}>
            {t("prescriptions.filterStatus")}
          </Text>
          <EnumSelect
            size="sm"
            value={statusFilter}
            onChange={setStatusFilter}
            placeholder={t("prescriptions.filterAll")}
            items={[
              { value: "", label: t("prescriptions.filterAll") },
              ...ALL_STATUSES.map((s) => ({
                value: s as string,
                label: t(`prescriptions.states.${s.toLowerCase()}`),
              })),
            ]}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
          />
        </Box>
        <Box minW="240px">
          <Text fontSize="xs" color="fg.muted" mb={1}>
            {t("prescriptions.customer")}
          </Text>
          <SearchableSelect
            size="sm"
            value={customerFilter}
            onChange={setCustomerFilter}
            loadOptions={searchCustomers}
            itemToString={(c) => c.name}
            itemToValue={(c) => c.id}
            selectedLabel={customerNameById.get(customerFilter)}
            placeholder={t("prescriptions.filterAll")}
          />
        </Box>
      </Flex>

      {rxQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("prescriptions.rxNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.customer")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.issuerName")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.issuedAt")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.expiresAt")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.filterStatus")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.items")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {(rxQ.data ?? []).map((rx) => {
              const dispensedAny = rx.items.some((it) => it.dispensedQty > 0);
              return (
                <Table.Row key={rx.id}>
                  <Table.Cell fontFamily="mono">{rx.rxNo}</Table.Cell>
                  <Table.Cell>{customerNameById.get(rx.customerId) ?? rx.customerId}</Table.Cell>
                  <Table.Cell>{rx.issuerName}</Table.Cell>
                  <Table.Cell>{rx.issuedAt}</Table.Cell>
                  <Table.Cell>{rx.expiresAt}</Table.Cell>
                  <Table.Cell>
                    <Badge colorPalette={statusBadge(rx.status)}>
                      {t(`prescriptions.states.${rx.status.toLowerCase()}`)}
                    </Badge>
                  </Table.Cell>
                  <Table.Cell>{rx.items.length}</Table.Cell>
                  <Table.Cell>
                    <HStack gap={1}>
                      {rx.status === "ACTIVE" && !dispensedAny && (
                        <Button size="xs" variant="ghost" onClick={() => setEditing(rx)}>
                          <Pencil size={14} />
                          {t("common.edit")}
                        </Button>
                      )}
                      {rx.status === "ACTIVE" && (
                        <Button
                          size="xs"
                          variant="ghost"
                          onClick={() => {
                            if (window.confirm(t("prescriptions.confirmVoid"))) {
                              voidMut.mutate(rx.id);
                            }
                          }}
                        >
                          <X size={14} />
                          {t("prescriptions.void")}
                        </Button>
                      )}
                    </HStack>
                  </Table.Cell>
                </Table.Row>
              );
            })}
            {(rxQ.data?.length ?? 0) === 0 && (
              <Table.Row>
                <Table.Cell colSpan={8}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("common.noResults")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      )}

      <PrescriptionDrawer
        open={createOpen}
        editing={null}
        onClose={() => setCreateOpen(false)}
      />
      <PrescriptionDrawer
        open={!!editing}
        editing={editing}
        onClose={() => setEditing(null)}
      />
    </Box>
  );
}

function PrescriptionDrawer({
  open,
  editing,
  onClose,
}: {
  open: boolean;
  editing: Prescription | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const customersQ = useCustomersQuery(false);
  const medicinesQ = useMedicinesQuery(false);
  const createMut = useCreatePrescriptionMutation();
  const updateMut = useUpdatePrescriptionMutation();

  const [customerId, setCustomerId] = useState(editing?.customerId ?? "");
  const [issuerName, setIssuerName] = useState(editing?.issuerName ?? "");
  const [issuedAt, setIssuedAt] = useState(editing?.issuedAt ?? isoToday());
  const [expiresAt, setExpiresAt] = useState(editing?.expiresAt ?? "");
  const [note, setNote] = useState(editing?.note ?? "");
  const [lines, setLines] = useState<Line[]>(
    editing
      ? editing.items.map((it) => ({
          medicineId: it.medicineId,
          prescribedQty: it.prescribedQty,
          dosageInstructions: it.dosageInstructions,
          note: it.note,
        }))
      : [emptyLine],
  );

  // Reset form when the drawer opens or the editing target changes.
  useEffect(() => {
    if (!open) return;
    setCustomerId(editing?.customerId ?? "");
    setIssuerName(editing?.issuerName ?? "");
    setIssuedAt(editing?.issuedAt ?? isoToday());
    setExpiresAt(editing?.expiresAt ?? "");
    setNote(editing?.note ?? "");
    setLines(
      editing
        ? editing.items.map((it) => ({
            medicineId: it.medicineId,
            prescribedQty: it.prescribedQty,
            dosageInstructions: it.dosageInstructions,
            note: it.note,
          }))
        : [emptyLine],
    );
  }, [editing, open]);

  const addLine = () => setLines((cur) => [...cur, emptyLine]);
  const removeLine = (idx: number) => setLines((cur) => cur.filter((_, i) => i !== idx));
  const updateLine = (idx: number, patch: Partial<Line>) =>
    setLines((cur) => cur.map((l, i) => (i === idx ? { ...l, ...patch } : l)));

  const canSubmit =
    !!customerId &&
    issuerName.trim().length > 0 &&
    issuedAt.length > 0 &&
    lines.length > 0 &&
    lines.every((l) => l.medicineId && l.prescribedQty > 0);

  const submit = async () => {
    const payload = {
      customerId,
      issuerName: issuerName.trim(),
      issuedAt,
      expiresAt,
      note: note.trim(),
      items: lines.map((l) => ({
        medicineId: l.medicineId,
        prescribedQty: l.prescribedQty,
        dosageInstructions: l.dosageInstructions,
        note: l.note,
      })),
    };
    try {
      if (editing) {
        await updateMut.mutateAsync({ id: editing.id, ...payload });
        toast.success(t("common.save") + " ✓");
      } else {
        await createMut.mutateAsync(payload);
        toast.success(t("common.create") + " ✓");
      }
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  const isPending = createMut.isPending || updateMut.isPending;

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={editing ? t("prescriptions.editTitle") : t("prescriptions.addTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={isPending} disabled={!canSubmit}>
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <Stack gap={4}>
        <Box>
          <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
            {t("prescriptions.customer")} *
          </Text>
          <SearchableSelect
            value={customerId}
            onChange={setCustomerId}
            loadOptions={searchCustomers}
            itemToString={(c) => c.name}
            itemToValue={(c) => c.id}
            selectedLabel={
              (customersQ.data ?? []).find((c) => c.id === customerId)?.name
            }
            placeholder={t("prescriptions.selectCustomer")}
          />
        </Box>

        <Box>
          <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
            {t("prescriptions.issuerName")} *
          </Text>
          <Input value={issuerName} onChange={(e) => setIssuerName(e.target.value)} />
        </Box>

        <Flex gap={3}>
          <Box flex="1">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("prescriptions.issuedAt")} *
            </Text>
            <Input type="date" value={issuedAt} onChange={(e) => setIssuedAt(e.target.value)} />
          </Box>
          <Box flex="1">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("prescriptions.expiresAt")}
            </Text>
            <Input type="date" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} />
            <Text fontSize="xs" color="fg.muted" mt={1}>
              {t("prescriptions.expiresAtHelp")}
            </Text>
          </Box>
        </Flex>

        <Box>
          <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
            {t("prescriptions.note")}
          </Text>
          <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} />
        </Box>

        <Box>
          <HStack justify="space-between" mb={2}>
            <Heading size="sm">{t("prescriptions.items")}</Heading>
            <Button size="xs" variant="outline" onClick={addLine}>
              <Plus size={14} />
              {t("prescriptions.addLine")}
            </Button>
          </HStack>
          <Stack gap={3}>
            {lines.map((l, idx) => {
              const dispensed = editing?.items[idx]?.dispensedQty ?? 0;
              const isRx = (medicinesQ.data ?? []).find((m) => m.id === l.medicineId)
                ?.prescriptionRequired;
              return (
                <Box
                  key={idx}
                  borderWidth="1px"
                  borderRadius="md"
                  p={3}
                  bg="bg.subtle"
                  borderColor="border.muted"
                >
                  <Flex gap={2} wrap="wrap" align="flex-end">
                    <Box flex="2" minW="220px">
                      <Text fontSize="xs" color="fg.muted" mb={1}>
                        {t("prescriptions.selectMedicine")}
                      </Text>
                      <SearchableSelect
                        size="sm"
                        value={l.medicineId}
                        onChange={(v) => updateLine(idx, { medicineId: v })}
                        loadOptions={searchMedicines}
                        itemToString={(m) =>
                          `${m.sku} · ${m.name}${m.prescriptionRequired ? " (Rx)" : ""}`
                        }
                        itemToValue={(m) => m.id}
                        selectedLabel={(() => {
                          const m = (medicinesQ.data ?? []).find((x) => x.id === l.medicineId);
                          return m
                            ? `${m.sku} · ${m.name}${m.prescriptionRequired ? " (Rx)" : ""}`
                            : undefined;
                        })()}
                        placeholder={t("prescriptions.selectMedicine")}
                      />
                      {l.medicineId && isRx === false && (
                        <Text fontSize="xs" color="fg.muted" mt={1}>
                          (non-Rx)
                        </Text>
                      )}
                    </Box>
                    <Box w="120px">
                      <Text fontSize="xs" color="fg.muted" mb={1}>
                        {t("prescriptions.prescribedQty")}
                      </Text>
                      <Input
                        size="sm"
                        type="number"
                        value={l.prescribedQty}
                        onChange={(e) =>
                          updateLine(idx, { prescribedQty: parseInt(e.target.value, 10) || 0 })
                        }
                      />
                    </Box>
                    {editing && (
                      <Box w="90px">
                        <Text fontSize="xs" color="fg.muted" mb={1}>
                          {t("prescriptions.dispensedQty")}
                        </Text>
                        <Text fontSize="sm" fontFamily="mono" pt={1}>
                          {dispensed}
                        </Text>
                      </Box>
                    )}
                    <IconButton
                      aria-label="remove line"
                      size="xs"
                      variant="ghost"
                      onClick={() => removeLine(idx)}
                      disabled={lines.length === 1}
                    >
                      <Trash2 size={14} />
                    </IconButton>
                  </Flex>
                  <Box mt={2}>
                    <Text fontSize="xs" color="fg.muted" mb={1}>
                      {t("prescriptions.dosage")}
                    </Text>
                    <Input
                      size="sm"
                      value={l.dosageInstructions}
                      onChange={(e) => updateLine(idx, { dosageInstructions: e.target.value })}
                    />
                  </Box>
                </Box>
              );
            })}
          </Stack>
        </Box>

        {editing && (
          <Text fontSize="xs" color="fg.muted">
            {t("prescriptions.rxNo")}: {editing.rxNo} · {formatDate(editing.issuedAt)}
          </Text>
        )}
      </Stack>
    </EntityDrawer>
  );
}
