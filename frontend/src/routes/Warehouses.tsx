import {
  Badge,
  Box,
  Button,
  Flex,
  HStack,
  Input,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Archive, Pencil, Plus, Search, Star } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import EntityDrawer from "../components/EntityDrawer";
import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import type { Warehouse } from "../gen/warehouse_iface/v1/warehouse_pb";
import { usePageState } from "../lib/pagination";
import { toast } from "../lib/toaster";
import {
  useArchiveWarehouseMutation,
  useCreateWarehouseMutation,
  useSetGlobalDefaultWarehouseMutation,
  useUpdateWarehouseMutation,
  useWarehousesQuery,
} from "../queries/warehouses";

export default function Warehouses() {
  const { t } = useTranslation();
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Warehouse | null>(null);
  const archive = useArchiveWarehouseMutation();
  const setGlobalDefault = useSetGlobalDefaultWarehouseMutation();
  // Admin sees everything (incl. inactive); single filter, so the resetKey
  // is a constant — usePageState still threads page/pageSize state.
  const includeInactive = true;
  const [queryInput, setQueryInput] = useState("");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const h = setTimeout(() => setQuery(queryInput.trim()), 250);
    return () => clearTimeout(h);
  }, [queryInput]);
  const { page, setPage, pageSize, setPageSize } = usePageState(`${includeInactive}|${query}`);
  const warehousesQ = useWarehousesQuery({ includeInactive, page, pageSize, query });

  const onSetDefault = async (w: Warehouse) => {
    if (!confirm(t("warehouses.confirmSetDefault", { name: w.name }))) return;
    try {
      await setGlobalDefault.mutateAsync(w.id);
      toast.success(t("warehouses.setAsDefault") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("warehouses.title") }]}
        title={t("warehouses.title")}
        actions={
          <Button colorPalette="blue" onClick={() => setCreateOpen(true)}>
            <Plus size={16} />
            {t("common.add")}
          </Button>
        }
      />

      <Box mb={3} maxW="320px">
        <HStack gap={2}>
          <Search size={16} />
          <Input
            size="sm"
            placeholder={t("warehouses.searchPlaceholder")}
            value={queryInput}
            onChange={(e) => setQueryInput(e.target.value)}
          />
        </HStack>
      </Box>

      {warehousesQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("warehouses.code")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("warehouses.name")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("warehouses.address")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("warehouses.phone")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.active")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {warehousesQ.rows.map((w) => (
              <Table.Row key={w.id}>
                <Table.Cell fontFamily="mono">
                  <HStack gap={2}>
                    <Text>{w.code}</Text>
                    {w.isDefault && (
                      <Badge size="xs" colorPalette="blue">
                        {t("warehouses.default")}
                      </Badge>
                    )}
                  </HStack>
                </Table.Cell>
                <Table.Cell>{w.name}</Table.Cell>
                <Table.Cell>{w.address}</Table.Cell>
                <Table.Cell>{w.phone}</Table.Cell>
                <Table.Cell>{w.active ? t("common.yes") : t("common.no")}</Table.Cell>
                <Table.Cell>
                  <HStack gap={1}>
                    <Button size="xs" variant="ghost" onClick={() => setEditing(w)}>
                      <Pencil size={14} />
                      {t("common.edit")}
                    </Button>
                    {w.active && !w.isDefault && (
                      <Button
                        size="xs"
                        variant="ghost"
                        colorPalette="blue"
                        onClick={() => onSetDefault(w)}
                        loading={setGlobalDefault.isPending}
                      >
                        <Star size={14} />
                        {t("warehouses.setAsDefault")}
                      </Button>
                    )}
                    {w.active && !w.isDefault && (
                      <Button
                        size="xs"
                        variant="ghost"
                        colorPalette="red"
                        onClick={() => archive.mutate(w.id)}
                      >
                        <Archive size={14} />
                        {t("common.archive")}
                      </Button>
                    )}
                  </HStack>
                </Table.Cell>
              </Table.Row>
            ))}
            {warehousesQ.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={6}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("common.noResults")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      )}

      <Box mt={3}>
        <Pagination
          page={page}
          pageSize={pageSize}
          total={warehousesQ.total}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
        />
      </Box>

      <WarehouseDrawer open={createOpen} onClose={() => setCreateOpen(false)} />
      <WarehouseDrawer
        open={!!editing}
        warehouse={editing}
        onClose={() => setEditing(null)}
      />
    </Box>
  );
}

function WarehouseDrawer({
  open,
  warehouse,
  onClose,
}: {
  open: boolean;
  warehouse?: Warehouse | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const isEdit = !!warehouse;
  const create = useCreateWarehouseMutation();
  const update = useUpdateWarehouseMutation();
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [phone, setPhone] = useState("");

  // Prefill when the drawer opens for an existing warehouse.
  const [seededId, setSeededId] = useState<string | null>(null);
  if (open && warehouse && seededId !== warehouse.id) {
    setSeededId(warehouse.id);
    setCode(warehouse.code);
    setName(warehouse.name);
    setAddress(warehouse.address);
    setPhone(warehouse.phone);
  }
  if (!open && seededId !== null) setSeededId(null);

  const submit = async () => {
    try {
      if (isEdit && warehouse) {
        await update.mutateAsync({ id: warehouse.id, name, address, phone });
        toast.success(t("common.save") + " ✓");
      } else {
        await create.mutateAsync({ code, name, address, phone });
        toast.success(t("common.create") + " ✓");
        setCode("");
      }
      setName("");
      setAddress("");
      setPhone("");
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={isEdit ? t("warehouses.editTitle") : t("warehouses.addTitle")}
      footer={
        <HStack justify="space-between" w="100%">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={submit}
            loading={create.isPending || update.isPending}
            disabled={!code || !name}
          >
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <Stack gap={3}>
        <Field label={t("warehouses.code")} required>
          <Input value={code} onChange={(e) => setCode(e.target.value)} disabled={isEdit} />
        </Field>
        <Field label={t("warehouses.name")} required>
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label={t("warehouses.address")}>
          <Input value={address} onChange={(e) => setAddress(e.target.value)} />
        </Field>
        <Field label={t("warehouses.phone")}>
          <Input value={phone} onChange={(e) => setPhone(e.target.value)} />
        </Field>
      </Stack>
    </EntityDrawer>
  );
}

function Field({
  label,
  required,
  children,
}: {
  label: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Flex direction="column" gap={1}>
      <Text fontSize="sm" fontWeight="medium" color="fg.muted">
        {label}
        {required ? " *" : ""}
      </Text>
      {children}
    </Flex>
  );
}
