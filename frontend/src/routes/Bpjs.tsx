import {
  Badge,
  Box,
  Button,
  Dialog,
  Flex,
  HStack,
  Input,
  Portal,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";

import EnumSelect from "../components/EnumSelect";
import { Send, X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import { formatMoney, formatUnix } from "../lib/format";
import { usePageState } from "../lib/pagination";
import { toast } from "../lib/toaster";
import {
  useBpjsClaimsQuery,
  useResolveBpjsClaimMutation,
  useSubmitBpjsClaimMutation,
} from "../queries/bpjs";

const STATUS_BADGE: Record<string, string> = {
  DRAFT: "gray",
  SUBMITTED: "blue",
  APPROVED: "green",
  REJECTED: "red",
  PAID: "purple",
};

export default function Bpjs() {
  const { t } = useTranslation();
  const [statusFilter, setStatusFilter] = useState("");
  const [resolving, setResolving] = useState<string | null>(null);
  const { page, setPage, pageSize, setPageSize } = usePageState(statusFilter);
  const claimsQ = useBpjsClaimsQuery({ status: statusFilter, limit: pageSize, offset: page * pageSize });
  const submitMut = useSubmitBpjsClaimMutation();

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("bpjs.title") }]}
        title={t("bpjs.title")}
        description={t("bpjs.stubNote")}
      />

      <HStack mb={3} gap={3}>
        <Box w="220px">
          <Text fontSize="xs" color="fg.muted" mb={1}>
            {t("bpjs.filterStatus")}
          </Text>
          <EnumSelect
            size="sm"
            value={statusFilter}
            onChange={setStatusFilter}
            placeholder={t("bpjs.filterAll")}
            items={[
              { value: "", label: t("bpjs.filterAll") },
              ...Object.keys(STATUS_BADGE).map((s) => ({ value: s, label: s })),
            ]}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
          />
        </Box>
      </HStack>

      {claimsQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("bpjs.bpjsNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("bpjs.saleId")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("bpjs.amount")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("bpjs.status")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("bpjs.submittedAt")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("bpjs.externalRef")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {claimsQ.rows.map((c) => (
              <Table.Row key={c.id}>
                <Table.Cell fontFamily="mono">{c.bpjsNo}</Table.Cell>
                <Table.Cell fontFamily="mono">{c.saleId.slice(0, 8)}</Table.Cell>
                <Table.Cell fontFamily="mono">{formatMoney(Number(c.amount))}</Table.Cell>
                <Table.Cell>
                  <Badge colorPalette={STATUS_BADGE[c.status] ?? "gray"}>{c.status}</Badge>
                </Table.Cell>
                <Table.Cell>{c.submittedAt ? formatUnix(c.submittedAt) : "—"}</Table.Cell>
                <Table.Cell fontFamily="mono">{c.externalRef || "—"}</Table.Cell>
                <Table.Cell>
                  <HStack gap={1}>
                    {c.status === "DRAFT" && (
                      <Button
                        size="xs"
                        variant="ghost"
                        loading={submitMut.isPending}
                        onClick={async () => {
                          try {
                            await submitMut.mutateAsync(c.id);
                            toast.success(t("bpjs.submitted"));
                          } catch {
                            /* */
                          }
                        }}
                      >
                        <Send size={14} />
                        {t("bpjs.submit")}
                      </Button>
                    )}
                    {(c.status === "SUBMITTED" || c.status === "APPROVED") && (
                      <Button size="xs" variant="ghost" onClick={() => setResolving(c.id)}>
                        {t("bpjs.resolve")}
                      </Button>
                    )}
                  </HStack>
                </Table.Cell>
              </Table.Row>
            ))}
            {claimsQ.rows.length === 0 && (
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

      <Box mt={3}>
        <Pagination
          page={page}
          pageSize={pageSize}
          total={claimsQ.total}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
        />
      </Box>

      <ResolveDialog id={resolving} onClose={() => setResolving(null)} />
    </Box>
  );
}

function ResolveDialog({ id, onClose }: { id: string | null; onClose: () => void }) {
  const { t } = useTranslation();
  const resolveMut = useResolveBpjsClaimMutation();
  const [status, setStatus] = useState("APPROVED");
  const [ref, setRef] = useState("");
  const [note, setNote] = useState("");
  if (!id) return null;
  const submit = async () => {
    try {
      await resolveMut.mutateAsync({ id, status, externalRef: ref, note });
      toast.success(t("common.save") + " ✓");
      onClose();
    } catch {
      /* */
    }
  };
  return (
    <Dialog.Root open={!!id} onOpenChange={(d) => !d.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("bpjs.resolveTitle")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <Button variant="ghost" size="sm">
                  <X size={16} />
                </Button>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                <Box>
                  <Text fontSize="xs" color="fg.muted" mb={1}>
                    {t("bpjs.status")}
                  </Text>
                  <EnumSelect
                    value={status}
                    onChange={setStatus}
                    items={[
                      { value: "APPROVED", label: "APPROVED" },
                      { value: "REJECTED", label: "REJECTED" },
                      { value: "PAID", label: "PAID" },
                    ]}
                    itemToString={(o) => o.label}
                    itemToValue={(o) => o.value}
                  />
                </Box>
                <Box>
                  <Text fontSize="xs" color="fg.muted" mb={1}>
                    {t("bpjs.externalRef")}
                  </Text>
                  <Input value={ref} onChange={(e) => setRef(e.target.value)} />
                </Box>
                <Box>
                  <Text fontSize="xs" color="fg.muted" mb={1}>
                    {t("bpjs.note")}
                  </Text>
                  <Input value={note} onChange={(e) => setNote(e.target.value)} />
                </Box>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <Flex gap={2} justify="flex-end" w="100%">
                <Button variant="ghost" onClick={onClose}>
                  {t("common.cancel")}
                </Button>
                <Button colorPalette="blue" onClick={submit} loading={resolveMut.isPending}>
                  {t("common.save")}
                </Button>
              </Flex>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
