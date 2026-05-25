import { useEffect, useState } from "react";
import { Box, Button, HStack, Input, Spinner, Stack, Table, Text } from "@chakra-ui/react";
import { Plus, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import ExportButton from "../../components/ExportButton";
import PageHeader from "../../components/PageHeader";
import Pagination from "../../components/Pagination";
import { downloadCsv } from "../../lib/csv";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { fetchMedicinesForExport, useMedicinesQuery } from "../../queries/medicines";
import { CreateMedicineDialog } from "./medicineDrawers";

export default function Medicines() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [createOpen, setCreateOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");

  // Debounce the search box (250ms) into the query that drives the request.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const { page, setPage, pageSize, setPageSize } = usePageState(query);
  const medicinesQ = useMedicinesQuery({ query, page, pageSize });

  const onExport = async () => {
    const rows = await fetchMedicinesForExport({ query });
    downloadCsv(
      `medicines-${new Date().toISOString().slice(0, 10)}.csv`,
      rows.map((m) => ({
        sku: m.sku,
        name: m.name,
        manufacturer: m.manufacturer,
        unit: m.unit,
        unitPrice: Number(m.unitPrice),
        ready: Number(m.readyStock),
        onOrder: Number(m.onOrderStock),
        rx: m.prescriptionRequired ? t("common.yes") : t("common.no"),
      })),
      [
        { key: "sku", header: t("inventory.medicines.sku") },
        { key: "name", header: t("inventory.medicines.name") },
        { key: "manufacturer", header: t("inventory.medicines.manufacturer") },
        { key: "unit", header: t("inventory.medicines.unit") },
        { key: "unitPrice", header: t("inventory.medicines.unitPrice") },
        { key: "ready", header: t("inventory.medicines.readyStock") },
        { key: "onOrder", header: t("inventory.medicines.onOrder") },
        { key: "rx", header: t("inventory.medicines.rxShort") },
      ],
    );
  };

  return (
    <Box>
      <PageHeader breadcrumbs={[{ label: t("nav.medicines") }]} title={t("nav.medicines")} />
      <Stack gap={4}>
        <HStack justify="space-between" wrap="wrap" gap={2}>
          <Box position="relative">
            <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
              <Search size={14} />
            </Box>
            <Input
              size="sm"
              pl={7}
              width="280px"
              placeholder={t("inventory.medicines.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <HStack gap={2}>
            <ExportButton onExport={onExport} />
            <Button size="sm" colorPalette="blue" onClick={() => setCreateOpen(true)}>
              <Plus size={16} />
              {t("inventory.medicines.addTitle")}
            </Button>
          </HStack>
        </HStack>

        {medicinesQ.isLoading ? (
          <Box p={8} textAlign="center">
            <Spinner />
          </Box>
        ) : (
          <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>{t("inventory.medicines.sku")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.medicines.name")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.medicines.unit")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.medicines.unitPrice")}</Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">{t("inventory.medicines.readyStock")}</Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">{t("inventory.medicines.onOrder")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.medicines.rxShort")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {medicinesQ.rows.map((m) => (
                <Table.Row
                  key={m.id}
                  cursor="pointer"
                  _hover={{ bg: "bg.muted" }}
                  onClick={() => navigate(`/medicines/${m.id}`)}
                >
                  <Table.Cell fontFamily="mono">{m.sku}</Table.Cell>
                  <Table.Cell>{m.name}</Table.Cell>
                  <Table.Cell>{m.unit}</Table.Cell>
                  <Table.Cell>{formatMoney(m.unitPrice)}</Table.Cell>
                  <Table.Cell textAlign="end">{m.readyStock.toString()}</Table.Cell>
                  <Table.Cell textAlign="end" color="fg.muted">
                    {m.onOrderStock > 0n ? m.onOrderStock.toString() : "—"}
                  </Table.Cell>
                  <Table.Cell>{m.prescriptionRequired ? t("common.yes") : t("common.no")}</Table.Cell>
                </Table.Row>
              ))}
              {medicinesQ.rows.length === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={7}>
                    <Text color="fg.muted" textAlign="center" py={4}>
                      {t("common.noResults")}
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
          total={medicinesQ.total}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
        />

        <CreateMedicineDialog open={createOpen} onClose={() => setCreateOpen(false)} />
      </Stack>
    </Box>
  );
}
