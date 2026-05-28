import { Badge, Box, Grid, Heading, SimpleGrid, Spinner, Stack, Table, Text } from "@chakra-ui/react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";

import BackButton from "../components/BackButton";
import PageHeader from "../components/PageHeader";
import { SaleStatus } from "../gen/pos_iface/v1/sale_pb";
import { formatMoney, formatUnix } from "../lib/format";
import { useCustomerRefs, useMedicineRefs, useUserRefs } from "../queries/refs";
import { useSaleQuery } from "../queries/sales";

const PAYMENT_KEY: Record<number, string> = {
  0: "unspecified",
  1: "cash",
  2: "bpjs",
  3: "insuranceOther",
};

const STATUS_BADGE: Record<number, string> = {
  [SaleStatus.UNSPECIFIED]: "gray",
  [SaleStatus.DRAFT]: "gray",
  [SaleStatus.COMPLETED]: "green",
  [SaleStatus.VOIDED]: "red",
};

function statusKey(s: SaleStatus): string {
  switch (s) {
    case SaleStatus.DRAFT:
      return "draft";
    case SaleStatus.COMPLETED:
      return "completed";
    case SaleStatus.VOIDED:
      return "voided";
    default:
      return "unspecified";
  }
}

export default function OrderDetail() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const saleQ = useSaleQuery(id);

  const sale = saleQ.data;
  const customerIds = useMemo(() => (sale?.customerId ? [sale.customerId] : []), [sale]);
  const cashierIds = useMemo(() => (sale?.cashierUserId ? [sale.cashierUserId] : []), [sale]);
  const medicineIds = useMemo(
    () => (sale?.items ?? []).map((i) => i.medicineId).filter(Boolean),
    [sale],
  );
  const customerRefs = useCustomerRefs(customerIds);
  const cashierRefs = useUserRefs(cashierIds);
  const medicineRefs = useMedicineRefs(medicineIds);

  if (saleQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  if (!sale) {
    return (
      <Box>
        <BackButton to="/orders" />
        <Box p={8}>
          <Text color="fg.muted">{t("common.noResults")}</Text>
        </Box>
      </Box>
    );
  }

  const saleNo = sale.saleNo || sale.id.slice(0, 8);
  const customer = sale.customerId ? customerRefs.get(sale.customerId)?.name ?? "—" : "—";
  const createdBy =
    cashierRefs.get(sale.cashierUserId)?.name ||
    cashierRefs.get(sale.cashierUserId)?.email ||
    "—";
  const paymentLabel = t(`orders.payments.${PAYMENT_KEY[sale.paymentSource] ?? "unspecified"}`);
  const change = Number(sale.paidAmount) - Number(sale.total);

  return (
    <Box>
      <BackButton to="/orders" />
      <PageHeader
        breadcrumbs={[
          { label: t("orders.title"), to: "/orders" },
          { label: saleNo },
        ]}
        title={saleNo}
        actions={
          <Badge colorPalette={STATUS_BADGE[sale.status] ?? "gray"} size="lg">
            {t(`orders.states.${statusKey(sale.status)}`)}
          </Badge>
        }
      />

      <Stack gap={6}>
        <Section title={t("orders.detail.info")}>
          <SimpleGrid columns={{ base: 1, md: 2, lg: 3 }} gap={4}>
            <Field label={t("orders.date")} value={formatUnix(sale.createdAt)} />
            <Field label={t("orders.createdBy")} value={createdBy} />
            <Field label={t("orders.customer")} value={customer} />
            <Field label={t("orders.payment")} value={paymentLabel} />
            {sale.prescriptionId && (
              <Field
                label={t("orders.detail.prescription")}
                value={sale.prescriptionId.slice(0, 8)}
                mono
              />
            )}
            {sale.completedAt > 0n && (
              <Field
                label={t("orders.detail.completedAt")}
                value={formatUnix(sale.completedAt)}
              />
            )}
          </SimpleGrid>
        </Section>

        <Section title={t("orders.detail.totals")}>
          <Grid templateColumns={{ base: "1fr 1fr", md: "repeat(5, 1fr)" }} gap={3}>
            <MoneyTile label={t("orders.detail.subtotal")} value={Number(sale.subtotal)} />
            <MoneyTile label={t("orders.detail.cartDiscount")} value={Number(sale.cartDiscount)} />
            <MoneyTile label={t("orders.detail.total")} value={Number(sale.total)} accent />
            <MoneyTile label={t("orders.detail.paid")} value={Number(sale.paidAmount)} />
            <MoneyTile label={t("orders.detail.change")} value={change} />
          </Grid>
        </Section>

        {sale.taxInvoiceNo && (
          <Section title={t("orders.detail.tax")}>
            <SimpleGrid columns={{ base: 1, md: 2, lg: 3 }} gap={4}>
              <Field label={t("orders.detail.taxInvoiceNo")} value={sale.taxInvoiceNo} mono />
            </SimpleGrid>
          </Section>
        )}

        <Section title={t("orders.detail.items")}>
          <Box overflowX="auto">
            <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
              <Table.Header bg="bg.muted">
                <Table.Row>
                  <Table.ColumnHeader>{t("orders.detail.medicine")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("orders.detail.qty")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">{t("orders.detail.unitPrice")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">{t("orders.detail.lineDiscount")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">{t("orders.detail.lineTotal")}</Table.ColumnHeader>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {sale.items.map((it) => {
                  const name = medicineRefs.get(it.medicineId)?.name || it.medicineName || "—";
                  return (
                    <Table.Row key={it.id}>
                      <Table.Cell>{name}</Table.Cell>
                      <Table.Cell>
                        {String(it.qty)}{it.unitName ? ` ${it.unitName}` : ""}
                      </Table.Cell>
                      <Table.Cell textAlign="end" fontFamily="mono">
                        {formatMoney(Number(it.unitPriceSnapshot))}
                      </Table.Cell>
                      <Table.Cell textAlign="end" fontFamily="mono">
                        {formatMoney(Number(it.lineDiscount))}
                      </Table.Cell>
                      <Table.Cell textAlign="end" fontFamily="mono">
                        {formatMoney(Number(it.lineTotal))}
                      </Table.Cell>
                    </Table.Row>
                  );
                })}
                {sale.items.length === 0 && (
                  <Table.Row>
                    <Table.Cell colSpan={5}>
                      <Text color="fg.muted" textAlign="center" py={4}>
                        {t("common.noResults")}
                      </Text>
                    </Table.Cell>
                  </Table.Row>
                )}
              </Table.Body>
            </Table.Root>
          </Box>
        </Section>
      </Stack>
    </Box>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Box>
      <Heading size="sm" mb={3}>
        {title}
      </Heading>
      {children}
    </Box>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <Box>
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Text fontFamily={mono ? "mono" : undefined}>{value}</Text>
    </Box>
  );
}

function MoneyTile({ label, value, accent }: { label: string; value: number; accent?: boolean }) {
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderColor={accent ? "blue.300" : "border"} borderRadius="lg" p={3}>
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Text fontFamily="mono" fontSize="md" fontWeight="semibold" color={accent ? "blue.600" : "fg"}>
        {formatMoney(value)}
      </Text>
    </Box>
  );
}
