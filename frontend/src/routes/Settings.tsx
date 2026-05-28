import {
  Box,
  Button,
  Dialog,
  HStack,
  Heading,
  IconButton,
  Input,
  Portal,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Download, Trash2, X } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import PageHeader from "../components/PageHeader";
import { formatUnix } from "../lib/format";
import { toast } from "../lib/toaster";
import {
  useBackupsQuery,
  useCreateBackupMutation,
  useDeleteBackupMutation,
} from "../queries/backup";
import { useSettingsQuery, useUpdateSettingsMutation } from "../queries/settings";

export default function Settings() {
  const { t } = useTranslation();
  const q = useSettingsQuery();
  const save = useUpdateSettingsMutation();

  // Local mirror of the threshold input so the user can type freely before
  // saving. Seeded from the loaded settings.
  const [threshold, setThreshold] = useState<string>("");
  useEffect(() => {
    if (q.data) setThreshold(String(q.data.lowStockThreshold));
  }, [q.data]);

  const onSave = async () => {
    const n = Number(threshold);
    if (!Number.isFinite(n) || !Number.isInteger(n) || n < 0) {
      toast.error(t("settings.invalidThreshold"));
      return;
    }
    try {
      await save.mutateAsync({ lowStockThreshold: n });
      toast.success(t("common.save") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("nav.settings") }]}
        title={t("settings.title")}
      />
      {q.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Stack gap={4} maxW="md">
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium">
              {t("settings.lowStockThreshold")}
            </Text>
            <HStack gap={2}>
              <Input
                type="number"
                inputMode="numeric"
                min={0}
                width="120px"
                value={threshold}
                onChange={(e) => setThreshold(e.target.value)}
              />
              <Button
                colorPalette="blue"
                onClick={onSave}
                loading={save.isPending}
              >
                {t("common.save")}
              </Button>
            </HStack>
            <Text fontSize="xs" color="fg.muted">
              {t("settings.lowStockHelp")}
            </Text>
          </Stack>
        </Stack>
      )}

      <BackupsSection />
    </Box>
  );
}

// BackupsSection renders the OWNER-only "Create backup" button + a table of
// past backup_<timestamp>/ directories. Delete prompts a small confirm dialog.
function BackupsSection() {
  const { t } = useTranslation();
  const backups = useBackupsQuery();
  const create = useCreateBackupMutation();
  const del = useDeleteBackupMutation();

  // `pending` holds the backup name awaiting deletion confirmation.
  const [pending, setPending] = useState<string | null>(null);

  const onCreate = async () => {
    try {
      await create.mutateAsync();
      toast.success(t("settings.backups.createdToast"));
    } catch {
      /* global toaster handles it */
    }
  };

  const onConfirmDelete = async () => {
    if (!pending) return;
    try {
      await del.mutateAsync(pending);
      toast.success(t("settings.backups.deletedToast"));
    } catch {
      /* global toaster handles it */
    } finally {
      setPending(null);
    }
  };

  const rows = backups.data ?? [];

  return (
    <Box mt={10} maxW="3xl">
      <HStack justify="space-between" mb={2}>
        <Heading size="md">{t("settings.backups.title")}</Heading>
        <Button
          colorPalette="blue"
          size="sm"
          loading={create.isPending}
          onClick={onCreate}
        >
          {t("settings.backups.create")}
        </Button>
      </HStack>
      <Text fontSize="xs" color="fg.muted" mb={3}>
        {t("settings.backups.help")}
      </Text>

      {backups.isLoading ? (
        <Box p={6} textAlign="center">
          <Spinner size="sm" />
        </Box>
      ) : rows.length === 0 ? (
        <Box p={6} borderWidth="1px" borderRadius="md" textAlign="center">
          <Text fontSize="sm" color="fg.muted">
            {t("settings.backups.empty")}
          </Text>
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("settings.backups.name")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("settings.backups.created")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("settings.backups.size")}</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">
                {t("common.actions")}
              </Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {rows.map((b) => (
              <Table.Row key={b.name}>
                <Table.Cell fontFamily="mono">
                  <HStack gap={2}>
                    <Download size={14} />
                    <Text>{b.name}</Text>
                  </HStack>
                </Table.Cell>
                <Table.Cell>{formatUnix(b.createdAt)}</Table.Cell>
                <Table.Cell>{formatBytes(Number(b.sizeBytes))}</Table.Cell>
                <Table.Cell textAlign="end">
                  <IconButton
                    aria-label={t("common.delete")}
                    size="xs"
                    variant="ghost"
                    onClick={() => setPending(b.name)}
                  >
                    <Trash2 size={14} />
                  </IconButton>
                </Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Root>
      )}

      <Dialog.Root
        open={pending !== null}
        onOpenChange={(d) => {
          if (!d.open) setPending(null);
        }}
        size="sm"
      >
        <Portal>
          <Dialog.Backdrop />
          <Dialog.Positioner>
            <Dialog.Content>
              <Dialog.Header borderBottomWidth="1px">
                <HStack justify="space-between">
                  <Heading size="md">{t("settings.backups.confirmTitle")}</Heading>
                  <IconButton
                    aria-label="close"
                    variant="ghost"
                    size="sm"
                    onClick={() => setPending(null)}
                  >
                    <X size={18} />
                  </IconButton>
                </HStack>
              </Dialog.Header>
              <Dialog.Body>
                <Text fontSize="sm">
                  {t("settings.backups.confirmBody", { name: pending ?? "" })}
                </Text>
              </Dialog.Body>
              <Dialog.Footer borderTopWidth="1px">
                <HStack justify="flex-end" w="full" gap={2}>
                  <Button variant="ghost" onClick={() => setPending(null)}>
                    {t("common.cancel")}
                  </Button>
                  <Button
                    colorPalette="red"
                    loading={del.isPending}
                    onClick={onConfirmDelete}
                  >
                    {t("common.delete")}
                  </Button>
                </HStack>
              </Dialog.Footer>
            </Dialog.Content>
          </Dialog.Positioner>
        </Portal>
      </Dialog.Root>
    </Box>
  );
}

// Compact byte-size formatter — small enough to inline; not worth a lib helper.
function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v >= 100 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`;
}
