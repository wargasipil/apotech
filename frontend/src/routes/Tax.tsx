import {
  Box,
  Button,
  Flex,
  HStack,
  Heading,
  Input,
  Stack,
  Table,
  Tabs,
  Text,
} from "@chakra-ui/react";
import { Download, Plus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import { downloadCsv } from "../lib/csv";
import { formatMoney, formatUnix } from "../lib/format";
import { usePageState } from "../lib/pagination";
import { toast } from "../lib/toaster";
import { useImportNsfpMutation, useNsfpQuery, useTaxInvoicesQuery } from "../queries/tax";

export default function Tax() {
  const { t } = useTranslation();
  return (
    <Box>
      <PageHeader breadcrumbs={[{ label: t("tax.title") }]} title={t("tax.title")} />
      <Tabs.Root defaultValue="invoices" variant="line">
        <Tabs.List>
          <Tabs.Trigger value="invoices">{t("tax.invoicesTab")}</Tabs.Trigger>
          <Tabs.Trigger value="nsfp">{t("tax.nsfpTab")}</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="invoices">
          <InvoicesPanel />
        </Tabs.Content>
        <Tabs.Content value="nsfp">
          <NsfpPanel />
        </Tabs.Content>
      </Tabs.Root>
    </Box>
  );
}

function InvoicesPanel() {
  const { t } = useTranslation();
  const { page, setPage, pageSize, setPageSize } = usePageState("");
  const invoicesQ = useTaxInvoicesQuery({ limit: pageSize, offset: page * pageSize });
  const rows = invoicesQ.rows;
  const exportCsv = () => {
    downloadCsv(
      `tax-invoices-${new Date().toISOString().slice(0, 10)}.csv`,
      rows.map((r) => ({
        tax_invoice_no: r.taxInvoiceNo,
        sale_no: r.saleNo,
        issued_at: r.issuedAt ? new Date(Number(r.issuedAt) * 1000).toISOString() : "",
        customer: r.customerName,
        npwp: r.customerNpwp,
        dpp: String(r.dpp),
        ppn: String(r.ppn),
        total: String(r.total),
      })),
      [
        { key: "tax_invoice_no", header: "TaxInvoiceNo" },
        { key: "sale_no", header: "SaleNo" },
        { key: "issued_at", header: "IssuedAt" },
        { key: "customer", header: "Customer" },
        { key: "npwp", header: "NPWP" },
        { key: "dpp", header: "DPP" },
        { key: "ppn", header: "PPN" },
        { key: "total", header: "Total" },
      ],
    );
  };
  return (
    <Stack gap={3} mt={4}>
      <HStack justify="flex-end">
        <Button size="sm" variant="outline" onClick={exportCsv} disabled={rows.length === 0}>
          <Download size={14} />
          {t("tax.exportCsv")}
        </Button>
      </HStack>
      <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
        <Table.Header bg="bg.muted">
          <Table.Row>
            <Table.ColumnHeader>{t("tax.invoiceNo")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("tax.saleNo")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("tax.issuedAt")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("tax.customer")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("tax.npwp")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("tax.dpp")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("tax.ppn")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("tax.total")}</Table.ColumnHeader>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {rows.map((r) => (
            <Table.Row key={r.saleId}>
              <Table.Cell fontFamily="mono">{r.taxInvoiceNo}</Table.Cell>
              <Table.Cell fontFamily="mono">{r.saleNo}</Table.Cell>
              <Table.Cell>{r.issuedAt ? formatUnix(r.issuedAt) : ""}</Table.Cell>
              <Table.Cell>{r.customerName}</Table.Cell>
              <Table.Cell fontFamily="mono">{r.customerNpwp}</Table.Cell>
              <Table.Cell fontFamily="mono">{formatMoney(Number(r.dpp))}</Table.Cell>
              <Table.Cell fontFamily="mono">{formatMoney(Number(r.ppn))}</Table.Cell>
              <Table.Cell fontFamily="mono">{formatMoney(Number(r.total))}</Table.Cell>
            </Table.Row>
          ))}
          {rows.length === 0 && (
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
      <Pagination
        page={page}
        pageSize={pageSize}
        total={invoicesQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />
    </Stack>
  );
}

function NsfpPanel() {
  const { t } = useTranslation();
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [year, setYear] = useState(new Date().getFullYear());
  const importMut = useImportNsfpMutation();
  const { page, setPage, pageSize, setPageSize } = usePageState("");
  const nsfpQ = useNsfpQuery({ unusedOnly: false, limit: pageSize, offset: page * pageSize });

  const submit = async () => {
    try {
      const res = await importMut.mutateAsync({
        startCode: start,
        endCode: end,
        fiscalYear: year,
      });
      toast.success(
        `${t("tax.imported")}: ${res.importedCount} · ${t("tax.skipped")}: ${res.skippedCount}`,
      );
      setStart("");
      setEnd("");
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Stack gap={4} mt={4}>
      <Box borderWidth="1px" borderRadius="lg" p={4} bg="bg.subtle">
        <Heading size="sm" mb={3}>
          {t("tax.importTitle")}
        </Heading>
        <Flex gap={3} wrap="wrap">
          <Box flex="1" minW="200px">
            <Text fontSize="xs" color="fg.muted" mb={1}>
              {t("tax.startCode")}
            </Text>
            <Input
              size="sm"
              value={start}
              onChange={(e) => setStart(e.target.value)}
              placeholder="000.24.00000001"
            />
          </Box>
          <Box flex="1" minW="200px">
            <Text fontSize="xs" color="fg.muted" mb={1}>
              {t("tax.endCode")}
            </Text>
            <Input
              size="sm"
              value={end}
              onChange={(e) => setEnd(e.target.value)}
              placeholder="000.24.00000100"
            />
          </Box>
          <Box w="120px">
            <Text fontSize="xs" color="fg.muted" mb={1}>
              {t("tax.fiscalYear")}
            </Text>
            <Input
              size="sm"
              type="number"
              value={year}
              onChange={(e) => setYear(parseInt(e.target.value, 10) || year)}
            />
          </Box>
          <Box alignSelf="flex-end">
            <Button colorPalette="blue" size="sm" onClick={submit} loading={importMut.isPending}>
              <Plus size={14} />
              {t("tax.import")}
            </Button>
          </Box>
        </Flex>
      </Box>

      <HStack>
        <Text fontSize="sm" color="fg.muted">
          {t("tax.unusedTotal")}: <b>{nsfpQ.unusedTotal}</b>
        </Text>
      </HStack>

      <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
        <Table.Header bg="bg.muted">
          <Table.Row>
            <Table.ColumnHeader>{t("tax.code")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("tax.fiscalYear")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("tax.used")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("tax.saleNo")}</Table.ColumnHeader>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {nsfpQ.rows.map((n) => (
            <Table.Row key={n.id}>
              <Table.Cell fontFamily="mono">{n.code}</Table.Cell>
              <Table.Cell>{n.fiscalYear}</Table.Cell>
              <Table.Cell>{n.usedAt ? formatUnix(n.usedAt) : "—"}</Table.Cell>
              <Table.Cell fontFamily="mono">{n.saleId.slice(0, 8) || "—"}</Table.Cell>
            </Table.Row>
          ))}
          {nsfpQ.rows.length === 0 && (
            <Table.Row>
              <Table.Cell colSpan={4}>
                <Text color="fg.muted" textAlign="center" py={4}>
                  {t("common.noResults")}
                </Text>
              </Table.Cell>
            </Table.Row>
          )}
        </Table.Body>
      </Table.Root>
      <Pagination
        page={page}
        pageSize={pageSize}
        total={nsfpQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />
    </Stack>
  );
}
