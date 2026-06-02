import { useEffect, useMemo, useState } from "react";
import { Box, Button, HStack, Input, Spinner, Stack, Table, Text } from "@chakra-ui/react";
import { Plus, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import DatePickerField from "../../components/DatePicker";
import ExportButton from "../../components/ExportButton";
import PageHeader from "../../components/PageHeader";
import Pagination from "../../components/Pagination";
import StockUnitPopover from "../../components/StockUnitPopover";
import { downloadCsv } from "../../lib/csv";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { formatStock, unitGroupsFromCatalog } from "../../lib/stockUnit";
import { fetchMedicinesForExport, useMedicinesQuery } from "../../queries/medicines";
import { useUnitBasesQuery } from "../../queries/units";
import { usePreferencesStore } from "../../stores/preferences";
import { CreateMedicineDialog } from "./medicineDrawers";

export default function Medicines() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [createOpen, setCreateOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  const [opnameBefore, setOpnameBefore] = useState("");

  // Debounce the search box (250ms) into the query that drives the request.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${query}|${opnameBefore}`,
  );
  const medicinesQ = useMedicinesQuery({ query, opnameBefore, page, pageSize });
  const stockUnitsByBase = usePreferencesStore((s) => s.medicineStockUnitsByBase);
  const setStockUnitByBase = usePreferencesStore((s) => s.setMedicineStockUnitByBase);
  const unitsQ = useUnitBasesQuery();
  const stockUnitGroups = useMemo(
    () => unitGroupsFromCatalog(unitsQ.data ?? [], medicinesQ.rows),
    [unitsQ.data, medicinesQ.rows],
  );

  const onExport = async () => {
    const rows = await fetchMedicinesForExport({ query, opnameBefore });
    downloadCsv(
      `medicines-${new Date().toISOString().slice(0, 10)}.csv`,
      rows.map((m) => ({
        sku: m.sku,
        name: m.name,
        unit: m.unit,
        unitPrice: Number(m.unitPrice),
        ready: Number(m.readyStock),
        onOrder: Number(m.onOrderStock),
        lastOpname: m.lastStocktakeDate || "",
        rx: m.prescriptionRequired ? t("common.yes") : t("common.no"),
      })),
      [
        { key: "sku", header: t("inventory.medicines.sku") },
        { key: "name", header: t("inventory.medicines.name") },
        { key: "unit", header: t("inventory.medicines.unit") },
        { key: "unitPrice", header: t("inventory.medicines.unitPrice") },
        { key: "ready", header: t("inventory.medicines.readyStock") },
        { key: "onOrder", header: t("inventory.medicines.onOrder") },
        { key: "lastOpname", header: t("inventory.medicines.lastStocktake") },
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
            <Text fontSize="sm" color="fg.muted">
              {t("inventory.medicines.opnameBefore")}
            </Text>
            <Box width="160px">
              <DatePickerField
                size="sm"
                value={opnameBefore}
                onChange={setOpnameBefore}
              />
            </Box>
            <StockUnitPopover
              byBase={stockUnitsByBase}
              onChangeBase={setStockUnitByBase}
              groups={stockUnitGroups}
            />
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
                <Table.ColumnHeader>{t("inventory.medicines.lastStocktake")}</Table.ColumnHeader>
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
                  <Table.Cell textAlign="end">
                    {formatStock(m.readyStock, m.units, m.unit, stockUnitsByBase)}
                  </Table.Cell>
                  <Table.Cell textAlign="end" color="fg.muted">
                    {m.onOrderStock > 0n
                      ? formatStock(m.onOrderStock, m.units, m.unit, stockUnitsByBase)
                      : "—"}
                  </Table.Cell>
                  <Table.Cell color={m.lastStocktakeDate ? "fg" : "fg.muted"}>
                    {m.lastStocktakeDate || "—"}
                  </Table.Cell>
                  <Table.Cell>{m.prescriptionRequired ? t("common.yes") : t("common.no")}</Table.Cell>
                </Table.Row>
              ))}
              {medicinesQ.rows.length === 0 && (
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
