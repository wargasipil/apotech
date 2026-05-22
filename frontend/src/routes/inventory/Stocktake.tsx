import { useState } from "react";
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
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import Pagination from "../../components/Pagination";
import { formatUnix } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import {
  useStartStocktakeMutation,
  useStocktakesQuery,
} from "../../queries/stocktake";

function statusPalette(status: string): string {
  switch (status) {
    case "DRAFT":
      return "yellow";
    case "COMPLETED":
      return "green";
    case "VOIDED":
      return "gray";
    default:
      return "gray";
  }
}

export default function Stocktake() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { page, setPage, pageSize, setPageSize } = usePageState("");
  const listQ = useStocktakesQuery({ page, pageSize });
  const startMut = useStartStocktakeMutation();
  const [creating, setCreating] = useState(false);

  const onStart = async () => {
    setCreating(true);
    try {
      const res = await startMut.mutateAsync({
        name: defaultSessionName(t),
      });
      toast.success(t("inventory.stocktake.createdToast"));
      navigate(`/inventory/stocktake/${res.session?.id ?? ""}`);
    } catch {
      /* toast handled globally */
    } finally {
      setCreating(false);
    }
  };

  return (
    <Stack gap={4}>
      <HStack justify="space-between">
        <Text fontSize="sm" color="fg.muted">
          {t("inventory.stocktake.intro")}
        </Text>
        <Button
          size="sm"
          colorPalette="blue"
          onClick={onStart}
          loading={creating}
        >
          <Plus size={16} />
          {t("inventory.stocktake.newSession")}
        </Button>
      </HStack>

      {listQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("inventory.stocktake.name")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.stocktake.status")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.stocktake.lines")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.stocktake.counted")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.stocktake.variances")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.stocktake.createdAt")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {listQ.rows.map((s) => (
              <Table.Row
                key={s.id}
                cursor="pointer"
                _hover={{ bg: "bg.muted" }}
                onClick={() => navigate(`/inventory/stocktake/${s.id}`)}
              >
                <Table.Cell>{s.name || s.id.slice(0, 8)}</Table.Cell>
                <Table.Cell>
                  <Badge colorPalette={statusPalette(s.status)}>
                    {t(`inventory.stocktake.statuses.${s.status.toLowerCase()}`, s.status)}
                  </Badge>
                </Table.Cell>
                <Table.Cell>{s.lineCount}</Table.Cell>
                <Table.Cell>{s.countedCount}</Table.Cell>
                <Table.Cell>{s.varianceCount}</Table.Cell>
                <Table.Cell>{formatUnix(s.createdAt)}</Table.Cell>
              </Table.Row>
            ))}
            {listQ.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={6}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("inventory.stocktake.empty")}
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
        total={listQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />
    </Stack>
  );
}

function defaultSessionName(t: (k: string, opts?: Record<string, unknown>) => string): string {
  const d = new Date();
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return t("inventory.stocktake.defaultName", {
    date: `${yyyy}-${mm}-${dd}`,
    defaultValue: `Stocktake ${yyyy}-${mm}-${dd}`,
  });
}
