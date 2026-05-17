import { Box, Spinner, Stack, Switch, Table, Text } from "@chakra-ui/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { formatMoney } from "../../lib/format";
import { useSupplierBalancesQuery } from "../../queries/purchasing";

export default function SuppliersLedger() {
  const { t } = useTranslation();
  const [onlyOutstanding, setOnlyOutstanding] = useState(true);
  const balancesQ = useSupplierBalancesQuery({ onlyOutstanding });

  return (
    <Stack gap={4}>
      <Switch.Root
        checked={onlyOutstanding}
        onCheckedChange={(d) => setOnlyOutstanding(d.checked)}
      >
        <Switch.HiddenInput />
        <Switch.Control />
        <Switch.Label>{t("purchasing.outstanding")} {">"} 0</Switch.Label>
      </Switch.Root>

      {balancesQ.isLoading ? (
        <Box p={6} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("purchasing.supplier")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.totalOrdered")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.totalPaid")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.outstanding")}</Table.ColumnHeader>
              <Table.ColumnHeader>Open POs</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {(balancesQ.data ?? []).map((b) => (
              <Table.Row key={b.supplierId}>
                <Table.Cell>{b.supplierName}</Table.Cell>
                <Table.Cell fontFamily="mono">{formatMoney(Number(b.orderedTotal))}</Table.Cell>
                <Table.Cell fontFamily="mono">{formatMoney(Number(b.paidTotal))}</Table.Cell>
                <Table.Cell fontFamily="mono">{formatMoney(Number(b.outstanding))}</Table.Cell>
                <Table.Cell>{b.openPoCount}</Table.Cell>
              </Table.Row>
            ))}
            {(balancesQ.data?.length ?? 0) === 0 && (
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
      )}
    </Stack>
  );
}
