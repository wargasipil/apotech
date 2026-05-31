import {
  Badge,
  Box,
  Button,
  HStack,
  Input,
  Spinner,
  Table,
  Text,
} from "@chakra-ui/react";
import { Archive, Pencil, Plus, Search, Star } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import WarehouseDrawer from "../components/WarehouseDrawer";
import type { Warehouse } from "../gen/warehouse_iface/v1/warehouse_pb";
import { usePageState } from "../lib/pagination";
import { toast } from "../lib/toaster";
import {
  useArchiveWarehouseMutation,
  useSetGlobalDefaultWarehouseMutation,
  useWarehousesQuery,
} from "../queries/warehouses";

export default function Warehouses() {
  const { t } = useTranslation();
  const navigate = useNavigate();
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
              <Table.Row
                key={w.id}
                cursor="pointer"
                _hover={{ bg: "bg.muted" }}
                onClick={() => navigate(`/warehouses/${w.id}`)}
              >
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
                <Table.Cell onClick={(e) => e.stopPropagation()}>
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

