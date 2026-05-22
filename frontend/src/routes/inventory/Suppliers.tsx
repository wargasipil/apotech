import { useState } from "react";
import {
  Box,
  Button,
  HStack,
  Spinner,
  Stack,
  Switch,
  Table,
  Text,
} from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { Archive, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import FormField from "../../components/FormField";
import Pagination from "../../components/Pagination";
import { Supplier } from "../../gen/inventory_iface/v1/supplier_pb";
import {
  useArchiveSupplierMutation,
  useCreateSupplierMutation,
  useSuppliersQuery,
} from "../../queries/suppliers";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";

const Schema = z.object({
  code: z.string().min(1),
  name: z.string().min(1),
  contactEmail: z.string().email().or(z.literal("")),
  phone: z.string(),
});
type FormValues = z.infer<typeof Schema>;

export default function Suppliers() {
  const { t } = useTranslation();
  const [includeInactive, setIncludeInactive] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const { page, setPage, pageSize, setPageSize } = usePageState(String(includeInactive));
  const suppliersQ = useSuppliersQuery({ includeInactive, page, pageSize });

  return (
    <Stack gap={4}>
      <HStack justify="space-between">
        <Switch.Root
          checked={includeInactive}
          onCheckedChange={(d) => setIncludeInactive(d.checked)}
        >
          <Switch.HiddenInput />
          <Switch.Control />
          <Switch.Label>{t("common.showArchived")}</Switch.Label>
        </Switch.Root>
        <Button size="sm" colorPalette="blue" onClick={() => setDrawerOpen(true)}>
          <Plus size={16} />
          {t("inventory.suppliers.addTitle")}
        </Button>
      </HStack>

      {suppliersQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("inventory.suppliers.code")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.suppliers.name")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.suppliers.email")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.suppliers.phone")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.active")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {suppliersQ.rows.map((s) => (
              <Row key={s.id} supplier={s} />
            ))}
            {suppliersQ.rows.length === 0 && (
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

      <Pagination
        page={page}
        pageSize={pageSize}
        total={suppliersQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <CreateDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </Stack>
  );
}

function Row({ supplier }: { supplier: Supplier }) {
  const { t } = useTranslation();
  const archive = useArchiveSupplierMutation();
  return (
    <Table.Row>
      <Table.Cell fontFamily="mono">{supplier.code}</Table.Cell>
      <Table.Cell>{supplier.name}</Table.Cell>
      <Table.Cell>{supplier.contactEmail}</Table.Cell>
      <Table.Cell>{supplier.phone}</Table.Cell>
      <Table.Cell>{supplier.active ? t("common.yes") : t("common.no")}</Table.Cell>
      <Table.Cell>
        {supplier.active && (
          <Button
            size="xs"
            variant="ghost"
            onClick={() => archive.mutate({ id: supplier.id })}
          >
            <Archive size={14} />
            {t("common.archive")}
          </Button>
        )}
      </Table.Cell>
    </Table.Row>
  );
}

function CreateDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateSupplierMutation();
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: { code: "", name: "", contactEmail: "", phone: "" },
  });

  const submit = form.handleSubmit(async (values) => {
    try {
      await create.mutateAsync({
        code: values.code,
        name: values.name,
        contactEmail: values.contactEmail,
        phone: values.phone,
      });
      toast.success(t("common.create") + " ✓");
      form.reset();
      onClose();
    } catch {
      /* toast handled globally */
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("inventory.suppliers.addTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending}>
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <Stack gap={4}>
          <FormField
            control={form.control}
            name="code"
            label={t("inventory.suppliers.code")}
            required
            autoFocus
          />
          <FormField
            control={form.control}
            name="name"
            label={t("inventory.suppliers.name")}
            required
          />
          <FormField
            control={form.control}
            name="contactEmail"
            label={t("inventory.suppliers.email")}
            type="email"
          />
          <FormField
            control={form.control}
            name="phone"
            label={t("inventory.suppliers.phone")}
          />
        </Stack>
      </form>
    </EntityDrawer>
  );
}
