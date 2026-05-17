import {
  Box,
  Button,
  Flex,
  HStack,
  Input,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Plus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import EntityDrawer from "../components/EntityDrawer";
import PageHeader from "../components/PageHeader";
import { toast } from "../lib/toaster";
import {
  useBranchesQuery,
  useCreateBranchMutation,
} from "../queries/branches";

export default function Branches() {
  const { t } = useTranslation();
  const [createOpen, setCreateOpen] = useState(false);
  const branchesQ = useBranchesQuery({ includeInactive: true });

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("branches.title") }]}
        title={t("branches.title")}
        actions={
          <Button colorPalette="blue" onClick={() => setCreateOpen(true)}>
            <Plus size={16} />
            {t("common.add")}
          </Button>
        }
      />

      {branchesQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("branches.code")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("branches.name")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("branches.address")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("branches.phone")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.active")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {(branchesQ.data ?? []).map((b) => (
              <Table.Row key={b.id}>
                <Table.Cell fontFamily="mono">{b.code}</Table.Cell>
                <Table.Cell>{b.name}</Table.Cell>
                <Table.Cell>{b.address}</Table.Cell>
                <Table.Cell>{b.phone}</Table.Cell>
                <Table.Cell>{b.active ? t("common.yes") : t("common.no")}</Table.Cell>
              </Table.Row>
            ))}
            {(branchesQ.data?.length ?? 0) === 0 && (
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
    </Box>
  );
}

function CreateDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateBranchMutation();
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [phone, setPhone] = useState("");

  const submit = async () => {
    try {
      await create.mutateAsync({ code, name, address, phone });
      toast.success(t("common.create") + " ✓");
      setCode("");
      setName("");
      setAddress("");
      setPhone("");
      onClose();
    } catch {
      /* */
    }
  };

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("branches.addTitle")}
      footer={
        <HStack justify="space-between" w="100%">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={submit}
            loading={create.isPending}
            disabled={!code || !name}
          >
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <Stack gap={3}>
        <Field label={t("branches.code")} required>
          <Input value={code} onChange={(e) => setCode(e.target.value)} />
        </Field>
        <Field label={t("branches.name")} required>
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label={t("branches.address")}>
          <Input value={address} onChange={(e) => setAddress(e.target.value)} />
        </Field>
        <Field label={t("branches.phone")}>
          <Input value={phone} onChange={(e) => setPhone(e.target.value)} />
        </Field>
      </Stack>
    </EntityDrawer>
  );
}

function Field({
  label,
  required,
  children,
}: {
  label: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Flex direction="column" gap={1}>
      <Text fontSize="sm" fontWeight="medium" color="fg.muted">
        {label}
        {required ? " *" : ""}
      </Text>
      {children}
    </Flex>
  );
}
