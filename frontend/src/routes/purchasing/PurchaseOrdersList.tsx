import {
  Badge,
  Box,
  Button,
  HStack,
  NativeSelect,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import {
  POStatus,
  type PurchaseOrder,
} from "../../gen/purchasing_iface/v1/order_pb";
import { formatMoney, formatDate } from "../../lib/format";
import { usePurchaseOrdersQuery } from "../../queries/purchasing";
import { useSuppliersQuery } from "../../queries/suppliers";

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
            <NativeSelect.Root size="sm" width="auto">
              <NativeSelect.Field
                value={String(statusFilter)}
                onChange={(e) => setStatusFilter(Number(e.target.value) as POStatus)}
              >
                <option value={POStatus.PO_STATUS_UNSPECIFIED}>{t("common.actions")} —</option>
                <option value={POStatus.PO_STATUS_DRAFT}>{t("purchasing.states.draft")}</option>
                <option value={POStatus.PO_STATUS_SENT}>{t("purchasing.states.sent")}</option>
                <option value={POStatus.PO_STATUS_PARTIALLY_RECEIVED}>
                  {t("purchasing.states.partiallyReceived")}
                </option>
                <option value={POStatus.PO_STATUS_RECEIVED}>{t("purchasing.states.received")}</option>
                <option value={POStatus.PO_STATUS_CLOSED}>{t("purchasing.states.closed")}</option>
                <option value={POStatus.PO_STATUS_VOIDED}>{t("purchasing.states.voided")}</option>
              </NativeSelect.Field>
              <NativeSelect.Indicator />
            </NativeSelect.Root>
          )}
          <NativeSelect.Root size="sm" width="auto">
            <NativeSelect.Field
              value={supplierFilter}
              onChange={(e) => setSupplierFilter(e.target.value)}
            >
              <option value="">{t("purchasing.supplier")} —</option>
              {(suppliersQ.data ?? []).map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </NativeSelect.Field>
            <NativeSelect.Indicator />
          </NativeSelect.Root>
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
