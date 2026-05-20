import {
  Badge,
  Box,
  Button,
  HStack,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import EnumSelect from "../../components/EnumSelect";
import SearchableSelect from "../../components/SearchableSelect";
import {
  POStatus,
  type PurchaseOrder,
} from "../../gen/purchasing_iface/v1/order_pb";
import { formatMoney, formatDate } from "../../lib/format";
import { usePurchaseOrdersQuery } from "../../queries/purchasing";
import { searchSuppliers, useSuppliersQuery } from "../../queries/suppliers";

type Props = { onlyOutstanding?: boolean };

const STATUS_BADGE_PALETTE: Record<POStatus, string> = {
  [POStatus.PO_STATUS_UNSPECIFIED]: "gray",
  [POStatus.PO_STATUS_DRAFT]: "gray",
  [POStatus.PO_STATUS_SENT]: "blue",
  [POStatus.PO_STATUS_PARTIALLY_RECEIVED]: "orange",
  [POStatus.PO_STATUS_RECEIVED]: "green",
  [POStatus.PO_STATUS_CLOSED]: "green",
  [POStatus.PO_STATUS_VOIDED]: "red",
};

export default function PurchaseOrdersList({ onlyOutstanding = false }: Props) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [statusFilter, setStatusFilter] = useState<POStatus>(POStatus.PO_STATUS_UNSPECIFIED);
  const [supplierFilter, setSupplierFilter] = useState("");

  const suppliersQ = useSuppliersQuery(false);
  const supplierName = useMemo(
    () => new Map((suppliersQ.data ?? []).map((s) => [s.id, s.name])),
    [suppliersQ.data],
  );

  const posQ = usePurchaseOrdersQuery({
    status: statusFilter,
    supplierId: supplierFilter,
    onlyOutstanding,
    limit: 200,
  });

  return (
    <Stack gap={4}>
      <HStack justify="space-between" wrap="wrap" gap={2}>
        <HStack gap={2}>
          {!onlyOutstanding && (
            <EnumSelect
              size="sm"
              width="180px"
              value={String(statusFilter)}
              onChange={(v) => setStatusFilter(Number(v) as POStatus)}
              items={[
                { value: String(POStatus.PO_STATUS_UNSPECIFIED), label: `${t("common.actions")} —` },
                { value: String(POStatus.PO_STATUS_DRAFT), label: t("purchasing.states.draft") },
                { value: String(POStatus.PO_STATUS_SENT), label: t("purchasing.states.sent") },
                { value: String(POStatus.PO_STATUS_PARTIALLY_RECEIVED), label: t("purchasing.states.partiallyReceived") },
                { value: String(POStatus.PO_STATUS_RECEIVED), label: t("purchasing.states.received") },
                { value: String(POStatus.PO_STATUS_CLOSED), label: t("purchasing.states.closed") },
                { value: String(POStatus.PO_STATUS_VOIDED), label: t("purchasing.states.voided") },
              ]}
              itemToString={(o) => o.label}
              itemToValue={(o) => o.value}
            />
          )}
          <SearchableSelect
            size="sm"
            width="220px"
            value={supplierFilter}
            onChange={setSupplierFilter}
            loadOptions={searchSuppliers}
            itemToString={(s) => s.name}
            itemToValue={(s) => s.id}
            selectedLabel={supplierName.get(supplierFilter)}
            placeholder={`${t("purchasing.supplier")} —`}
          />
        </HStack>
        <Button colorPalette="blue" onClick={() => navigate("/purchasing/new")}>
          <Plus size={16} />
          {t("purchasing.newPo")}
        </Button>
      </HStack>

      {posQ.isLoading ? (
        <Box p={6} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("purchasing.poNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.supplier")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.status")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.totalOrdered")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.outstanding")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("purchasing.expectedAt")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {(posQ.data ?? []).map((po: PurchaseOrder) => (
              <Table.Row
                key={po.id}
                onClick={() => navigate(`/purchasing/${po.id}`)}
                cursor="pointer"
                _hover={{ bg: "bg.muted" }}
              >
                <Table.Cell fontFamily="mono">{po.poNo || po.id.slice(0, 8)}</Table.Cell>
                <Table.Cell>{supplierName.get(po.supplierId) ?? po.supplierId.slice(0, 8)}</Table.Cell>
                <Table.Cell>
                  <Badge colorPalette={STATUS_BADGE_PALETTE[po.status]}>
                    {t(`purchasing.states.${statusKey(po.status)}`)}
                  </Badge>
                </Table.Cell>
                <Table.Cell fontFamily="mono">{formatMoney(Number(po.orderedTotal))}</Table.Cell>
                <Table.Cell fontFamily="mono">{formatMoney(Number(po.outstanding))}</Table.Cell>
                <Table.Cell>{po.expectedAt ? formatDate(po.expectedAt) : "—"}</Table.Cell>
              </Table.Row>
            ))}
            {(posQ.data?.length ?? 0) === 0 && (
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
    </Stack>
  );
}

function statusKey(s: POStatus): string {
  switch (s) {
    case POStatus.PO_STATUS_DRAFT:
      return "draft";
    case POStatus.PO_STATUS_SENT:
      return "sent";
    case POStatus.PO_STATUS_PARTIALLY_RECEIVED:
      return "partiallyReceived";
    case POStatus.PO_STATUS_RECEIVED:
      return "received";
    case POStatus.PO_STATUS_CLOSED:
      return "closed";
    case POStatus.PO_STATUS_VOIDED:
      return "voided";
    default:
      return "draft";
  }
}
