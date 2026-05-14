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
import { Archive, Pencil, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import EntityDrawer from "../components/EntityDrawer";
import FormField from "../components/FormField";
import PageHeader from "../components/PageHeader";
import { Customer } from "../gen/customer_iface/v1/customer_pb";
import { toast } from "../lib/toaster";
import {
  useArchiveCustomerMutation,
  useCreateCustomerMutation,
  useCustomersQuery,
  useUpdateCustomerMutation,
} from "../queries/customers";

const Schema = z.object({
  name: z.string().min(1),
  phone: z.string(),
  bpjsNo: z.string(),
  notes: z.string(),
});
type FormValues = z.infer<typeof Schema>;

export default function Customers() {
  const { t } = useTranslation();
  const [includeInactive, setIncludeInactive] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Customer | null>(null);
  const customersQ = useCustomersQuery(includeInactive);

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("customers.title") }]}
        title={t("customers.title")}
        actions={
          <Button colorPalette="blue" onClick={() => setCreateOpen(true)}>
            <Plus size={16} />
            {t("common.add")}
          </Button>
        }
      />

      <HStack mb={4}>
        <Switch.Root
          checked={includeInactive}
          onCheckedChange={(d) => setIncludeInactive(d.checked)}
        >
          <Switch.HiddenInput />
          <Switch.Control />
          <Switch.Label>{t("common.showArchived")}</Switch.Label>
        </Switch.Root>
      </HStack>

      {customersQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("customers.name")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("customers.phone")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("customers.bpjsNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.active")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {customersQ.data?.map((c) => (
              <Row key={c.id} customer={c} onEdit={() => setEditing(c)} />
            ))}
            {(customersQ.data?.length ?? 0) === 0 && (
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

      <CreateDrawer open={createOpen} onClose={() => setCreateOpen(false)} />
      <EditDrawer customer={editing} onClose={() => setEditing(null)} />
    </Box>
  );
}

function Row({ customer, onEdit }: { customer: Customer; onEdit: () => void }) {
  const { t } = useTranslation();
  const archive = useArchiveCustomerMutation();
  return (
    <Table.Row>
      <Table.Cell>{customer.name}</Table.Cell>
      <Table.Cell>{customer.phone}</Table.Cell>
      <Table.Cell>{customer.bpjsNo}</Table.Cell>
      <Table.Cell>{customer.active ? t("common.yes") : t("common.no")}</Table.Cell>
      <Table.Cell>
        <HStack gap={1}>
          <Button size="xs" variant="ghost" onClick={onEdit}>
            <Pencil size={14} />
            {t("common.edit")}
          </Button>
          {customer.active && (
            <Button
              size="xs"
              variant="ghost"
              onClick={() => archive.mutate({ id: customer.id })}
            >
              <Archive size={14} />
              {t("common.archive")}
            </Button>
          )}
        </HStack>
      </Table.Cell>
    </Table.Row>
  );
}

function CustomerForm({ form }: { form: ReturnType<typeof useForm<FormValues>> }) {
  const { t } = useTranslation();
  return (
    <Stack gap={4}>
      <FormField control={form.control} name="name" label={t("customers.name")} required autoFocus />
      <FormField control={form.control} name="phone" label={t("customers.phone")} />
      <FormField control={form.control} name="bpjsNo" label={t("customers.bpjsNo")} />
      <FormField control={form.control} name="notes" label={t("customers.notes")} />
    </Stack>
  );
}

function CreateDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateCustomerMutation();
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: { name: "", phone: "", bpjsNo: "", notes: "" },
  });

  const submit = form.handleSubmit(async (values) => {
    try {
      await create.mutateAsync(values);
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
      title={t("customers.addTitle")}
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
        <CustomerForm form={form} />
      </form>
    </EntityDrawer>
  );
}

function EditDrawer({ customer, onClose }: { customer: Customer | null; onClose: () => void }) {
  const { t } = useTranslation();
  const update = useUpdateCustomerMutation();
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    values: customer
      ? {
          name: customer.name,
          phone: customer.phone,
          bpjsNo: customer.bpjsNo,
          notes: customer.notes,
        }
      : undefined,
  });

  const submit = form.handleSubmit(async (values) => {
    if (!customer) return;
    try {
      await update.mutateAsync({ id: customer.id, ...values });
      toast.success(t("common.save") + " ✓");
      onClose();
    } catch {
      /* toast handled globally */
    }
  });

  return (
    <EntityDrawer
      open={!!customer}
      onClose={onClose}
      title={t("customers.editTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={update.isPending}>
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <CustomerForm form={form} />
      </form>
    </EntityDrawer>
  );
}
