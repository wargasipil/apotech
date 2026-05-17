import {
  Box,
  Button,
  Grid,
  HStack,
  Heading,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Download } from "lucide-react";
import { useTranslation } from "react-i18next";

import { formatMoney } from "../../lib/format";
import { downloadCsv } from "../../lib/csv";
import {
  useDaysOfStockQuery,
  useDeadStockQuery,
  useExpiryRiskQuery,
  useTurnoverQuery,
} from "../../queries/analytics";

export default function InventoryAnalytics() {
  const { t } = useTranslation();

  const turnoverQ = useTurnoverQuery({ periodDays: 30, limit: 25 });
  const deadQ = useDeadStockQuery({ noMovementDays: 60 });
  const daysQ = useDaysOfStockQuery({ sampleDays: 30 });
  const expiryQ = useExpiryRiskQuery();

  return (
    <Stack gap={6}>
      <Grid templateColumns={{ base: "1fr", md: "repeat(3, 1fr)" }} gap={4}>
        {(expiryQ.data?.buckets ?? []).map((b) => (
          <Box
            key={Number(b.windowDays)}
            bg="bg.subtle"
            borderWidth="1px"
            borderRadius="lg"
            p={4}
          >
            <Text fontSize="sm" color="fg.muted">
              {t("analytics.inventory.expiringIn")} {b.windowDays}d
            </Text>
            <Text fontSize="xl" fontWeight="semibold" fontFamily="mono">
              {String(b.qtyAtRisk)} {t("analytics.inventory.qtyAtRisk")}
            </Text>
            <Text fontSize="xs" color="fg.muted" mt={1}>
              {formatMoney(Number(b.valueAtRisk))}
            </Text>
          </Box>
        ))}
      </Grid>

      <Section
        title={t("analytics.inventory.turnover")}
        actions={
          <Button
            size="xs"
            variant="ghost"
            onClick={() =>
              downloadCsv(
                "turnover.csv",
                (turnoverQ.data?.rows ?? []).map((r) => ({
                  sku: r.sku,
                  medicine: r.medicineName,
                  sold: Number(r.soldQty),
                  avg_inv: Number(r.avgInventoryQty),
                  ratio: r.turnoverRatio.toFixed(2),
                })),
                [
                  { key: "sku", header: "SKU" },
                  { key: "medicine", header: "Medicine" },
                  { key: "sold", header: "Sold (30d)" },
                  { key: "avg_inv", header: "Avg inventory" },
                  { key: "ratio", header: "Turnover ratio" },
                ],
              )
            }
          >
            <Download size={14} />
            {t("analytics.exportCsv")}
          </Button>
        }
      >
        {turnoverQ.isLoading ? (
          <CenterSpinner />
        ) : (
          <Table.Root size="sm">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>SKU</Table.ColumnHeader>
                <Table.ColumnHeader>Medicine</Table.ColumnHeader>
                <Table.ColumnHeader>{t("analytics.inventory.sold")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("analytics.inventory.avgInventory")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("analytics.inventory.ratio")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {(turnoverQ.data?.rows ?? []).map((r) => (
                <Table.Row key={r.medicineId}>
                  <Table.Cell>{r.sku}</Table.Cell>
                  <Table.Cell>{r.medicineName}</Table.Cell>
                  <Table.Cell>{String(r.soldQty)}</Table.Cell>
                  <Table.Cell>{String(r.avgInventoryQty)}</Table.Cell>
                  <Table.Cell>{r.turnoverRatio.toFixed(2)}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        )}
      </Section>

      <Section
        title={t("analytics.inventory.daysRemaining")}
        actions={
          <Button
            size="xs"
            variant="ghost"
            onClick={() =>
              downloadCsv(
                "days-remaining.csv",
                (daysQ.data?.rows ?? []).map((r) => ({
                  sku: r.sku,
                  medicine: r.medicineName,
                  current: Number(r.currentQty),
                  avg_daily: r.avgDailyConsumption.toFixed(2),
                  days_left:
                    r.daysRemaining > 0 ? r.daysRemaining.toFixed(1) : "∞",
                })),
                [
                  { key: "sku", header: "SKU" },
                  { key: "medicine", header: "Medicine" },
                  { key: "current", header: "On hand" },
                  { key: "avg_daily", header: "Avg daily" },
                  { key: "days_left", header: "Days left" },
                ],
              )
            }
          >
            <Download size={14} />
            {t("analytics.exportCsv")}
          </Button>
        }
      >
        {daysQ.isLoading ? (
          <CenterSpinner />
        ) : (
          <Table.Root size="sm">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>SKU</Table.ColumnHeader>
                <Table.ColumnHeader>Medicine</Table.ColumnHeader>
                <Table.ColumnHeader>{t("analytics.inventory.currentQty")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("analytics.inventory.avgDaily")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("analytics.inventory.daysLeft")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {(daysQ.data?.rows ?? []).map((r) => (
                <Table.Row key={r.medicineId}>
                  <Table.Cell>{r.sku}</Table.Cell>
                  <Table.Cell>{r.medicineName}</Table.Cell>
                  <Table.Cell>{String(r.currentQty)}</Table.Cell>
                  <Table.Cell>{r.avgDailyConsumption.toFixed(2)}</Table.Cell>
                  <Table.Cell>
                    {r.daysRemaining > 0 ? r.daysRemaining.toFixed(1) : "∞"}
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        )}
      </Section>

      <Section title={t("analytics.inventory.deadStock")}>
        {deadQ.isLoading ? (
          <CenterSpinner />
        ) : (
          <Table.Root size="sm">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>SKU</Table.ColumnHeader>
                <Table.ColumnHeader>Medicine</Table.ColumnHeader>
                <Table.ColumnHeader>{t("analytics.inventory.currentQty")}</Table.ColumnHeader>
                <Table.ColumnHeader>Last sale</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {(deadQ.data?.rows ?? []).map((r) => (
                <Table.Row key={r.medicineId}>
                  <Table.Cell>{r.sku}</Table.Cell>
                  <Table.Cell>{r.medicineName}</Table.Cell>
                  <Table.Cell>{String(r.currentQty)}</Table.Cell>
                  <Table.Cell>
                    {r.lastSaleUnix > 0
                      ? new Date(Number(r.lastSaleUnix) * 1000).toLocaleDateString()
                      : "—"}
                  </Table.Cell>
                </Table.Row>
              ))}
              {(deadQ.data?.rows?.length ?? 0) === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={4}>
                    <Text color="fg.muted" textAlign="center" py={4}>
                      —
                    </Text>
                  </Table.Cell>
                </Table.Row>
              )}
            </Table.Body>
          </Table.Root>
        )}
      </Section>
    </Stack>
  );
}

function Section({
  title,
  actions,
  children,
}: {
  title: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
      <HStack justify="space-between" mb={3}>
        <Heading size="sm">{title}</Heading>
        {actions}
      </HStack>
      {children}
    </Box>
  );
}

function CenterSpinner() {
  return (
    <Box p={6} textAlign="center">
      <Spinner />
    </Box>
  );
}
