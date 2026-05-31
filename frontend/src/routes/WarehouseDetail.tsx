import {
  Badge,
  Box,
  Button,
  Dialog,
  Heading,
  HStack,
  IconButton,
  Portal,
  SimpleGrid,
  Spinner,
  Stack,
  Switch,
  Table,
  Text,
} from "@chakra-ui/react";
import { Archive, Pencil, Star, UserMinus, X } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import BackButton from "../components/BackButton";
import PageHeader from "../components/PageHeader";
import SearchableSelect from "../components/SearchableSelect";
import WarehouseDrawer from "../components/WarehouseDrawer";
import type { UserRef } from "../gen/user_iface/v1/users_pb";
import type { WarehouseUser } from "../gen/warehouse_iface/v1/warehouse_pb";
import { useAuth } from "../lib/auth";
import { toast } from "../lib/toaster";
import { searchUsers } from "../queries/users";
import {
  useArchiveWarehouseMutation,
  useGrantWarehouseAccessMutation,
  useRevokeWarehouseAccessMutation,
  useSetDefaultWarehouseMutation,
  useSetGlobalDefaultWarehouseMutation,
  useWarehouseQuery,
  useWarehouseUsersQuery,
} from "../queries/warehouses";

export default function WarehouseDetail() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { user: me } = useAuth();
  const { id = "" } = useParams();
  const [editing, setEditing] = useState(false);
  const [revoking, setRevoking] = useState<WarehouseUser | null>(null);

  const whQ = useWarehouseQuery(id);
  const usersQ = useWarehouseUsersQuery(id);
  const archive = useArchiveWarehouseMutation();
  const setGlobalDefault = useSetGlobalDefaultWarehouseMutation();
  const grant = useGrantWarehouseAccessMutation();
  const revoke = useRevokeWarehouseAccessMutation();
  const setUserDefault = useSetDefaultWarehouseMutation();

  const w = whQ.data;
  const users = usersQ.data ?? [];

  // Exclude users already in the access list from the picker's loadOptions
  // results (small set; client-side filter is fine after the server search).
  const alreadyAccessIds = useMemo(
    () => new Set(users.map((u) => u.userId)),
    [users],
  );
  const loadUsers = async (query: string) => {
    const found = await searchUsers(query);
    return found.filter((u) => !alreadyAccessIds.has(u.id));
  };

  if (whQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  if (!w) {
    return (
      <Box>
        <BackButton to="/warehouses" />
        <Box p={8}>
          <Text color="fg.muted">{t("common.noResults")}</Text>
        </Box>
      </Box>
    );
  }

  const onArchive = async () => {
    if (!confirm(t("warehouses.confirmArchive", { name: w.name }))) return;
    try {
      await archive.mutateAsync(w.id);
      toast.success(t("common.archive") + " ✓");
      navigate("/warehouses");
    } catch {
      /* toast handled globally */
    }
  };

  const onPromote = async () => {
    if (!confirm(t("warehouses.confirmSetDefault", { name: w.name }))) return;
    try {
      await setGlobalDefault.mutateAsync(w.id);
      toast.success(t("warehouses.setAsDefault") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };

  const onAddUser = async (newUser: UserRef | undefined) => {
    if (!newUser) return;
    try {
      await grant.mutateAsync({
        userId: newUser.id,
        warehouseId: w.id,
        isDefault: false,
      });
      toast.success(t("warehouses.addUser") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };

  const onSetUserDefault = async (row: WarehouseUser) => {
    if (row.isDefault) return; // disabled in UI; defensive
    try {
      await setUserDefault.mutateAsync({
        userId: row.userId,
        warehouseId: w.id,
      });
      toast.success(t("warehouses.defaultForUser") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };

  const onRevokeConfirmed = async () => {
    if (!revoking) return;
    try {
      await revoke.mutateAsync({
        userId: revoking.userId,
        warehouseId: w.id,
      });
      toast.success(t("warehouses.revokeAccess") + " ✓");
      setRevoking(null);
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Box>
      <BackButton to="/warehouses" />
      <PageHeader
        breadcrumbs={[
          { label: t("nav.warehouses"), to: "/warehouses" },
          { label: w.name },
        ]}
        title={w.name}
        actions={
          <HStack gap={2}>
            <Button size="sm" variant="ghost" onClick={() => setEditing(true)}>
              <Pencil size={14} />
              {t("common.edit")}
            </Button>
            {w.active && !w.isDefault && (
              <Button
                size="sm"
                variant="ghost"
                colorPalette="blue"
                onClick={onPromote}
                loading={setGlobalDefault.isPending}
              >
                <Star size={14} />
                {t("warehouses.setAsDefault")}
              </Button>
            )}
            {w.active && !w.isDefault && (
              <Button
                size="sm"
                variant="ghost"
                colorPalette="red"
                onClick={onArchive}
                loading={archive.isPending}
              >
                <Archive size={14} />
                {t("common.archive")}
              </Button>
            )}
          </HStack>
        }
      />

      <Stack gap={6}>
        {/* Info card */}
        <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
          <Heading size="sm" mb={3}>
            {t("warehouses.title")}
          </Heading>
          <SimpleGrid columns={{ base: 1, md: 3 }} gap={4}>
            <Field label={t("warehouses.code")} value={w.code} mono />
            <Field label={t("warehouses.name")} value={w.name} />
            <Field label={t("warehouses.address")} value={w.address || "—"} />
            <Field label={t("warehouses.phone")} value={w.phone || "—"} />
            <Field
              label={t("warehouses.default")}
              value={
                w.isDefault ? (
                  <Badge colorPalette="blue">{t("common.yes")}</Badge>
                ) : (
                  t("common.no")
                )
              }
            />
            <Field
              label={t("common.active")}
              value={
                w.active ? (
                  <Badge colorPalette="green">{t("common.yes")}</Badge>
                ) : (
                  <Badge colorPalette="red">{t("common.no")}</Badge>
                )
              }
            />
          </SimpleGrid>
        </Box>

        {/* Users with access section */}
        <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
          <HStack justify="space-between" mb={3} wrap="wrap" gap={3}>
            <Heading size="sm">{t("warehouses.usersSection")}</Heading>
            <Box w={{ base: "100%", md: "320px" }}>
              <SearchableSelect<UserRef>
                value=""
                onChange={() => {
                  /* selection handled via onSelectItem */
                }}
                onSelectItem={onAddUser}
                loadOptions={loadUsers}
                itemToString={(u) => `${u.email}${u.name ? ` · ${u.name}` : ""}`}
                itemToValue={(u) => u.id}
                placeholder={t("warehouses.addUser")}
              />
            </Box>
          </HStack>

          {usersQ.isLoading ? (
            <Box p={6} textAlign="center">
              <Spinner size="sm" />
            </Box>
          ) : users.length === 0 ? (
            <Text color="fg.muted" textAlign="center" py={4} fontSize="sm">
              {t("warehouses.noUsers")}
            </Text>
          ) : (
            <Table.Root size="sm" variant="line">
              <Table.Header bg="bg.muted">
                <Table.Row>
                  <Table.ColumnHeader>{t("users.email")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("users.name")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("users.role")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("warehouses.defaultForUser")}</Table.ColumnHeader>
                  <Table.ColumnHeader />
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {users.map((u) => {
                  const isSelf = me?.id === u.userId;
                  return (
                    <Table.Row key={u.userId}>
                      <Table.Cell>{u.email}</Table.Cell>
                      <Table.Cell>{u.name || "—"}</Table.Cell>
                      <Table.Cell>{u.role}</Table.Cell>
                      <Table.Cell>
                        <Switch.Root
                          checked={u.isDefault}
                          disabled={u.isDefault}
                          title={u.isDefault ? t("warehouses.defaultLockedTooltip") : undefined}
                          onCheckedChange={() => onSetUserDefault(u)}
                        >
                          <Switch.HiddenInput />
                          <Switch.Control />
                        </Switch.Root>
                      </Table.Cell>
                      <Table.Cell textAlign="end">
                        {!isSelf && (
                          <Button
                            size="xs"
                            variant="ghost"
                            colorPalette="red"
                            onClick={() => setRevoking(u)}
                          >
                            <UserMinus size={14} />
                            {t("warehouses.revokeAccess")}
                          </Button>
                        )}
                      </Table.Cell>
                    </Table.Row>
                  );
                })}
              </Table.Body>
            </Table.Root>
          )}
        </Box>
      </Stack>

      <WarehouseDrawer
        open={editing}
        warehouse={w}
        onClose={() => setEditing(false)}
      />

      {/* Revoke confirm dialog */}
      <Dialog.Root open={!!revoking} onOpenChange={(d) => !d.open && setRevoking(null)} size="sm">
        <Portal>
          <Dialog.Backdrop />
          <Dialog.Positioner>
            <Dialog.Content>
              <Dialog.Header>
                <Dialog.Title>{t("warehouses.revokeAccess")}</Dialog.Title>
                <Dialog.CloseTrigger asChild>
                  <IconButton aria-label="close" variant="ghost" size="sm">
                    <X size={16} />
                  </IconButton>
                </Dialog.CloseTrigger>
              </Dialog.Header>
              <Dialog.Body>
                <Text>
                  {t("warehouses.confirmRevoke", { email: revoking?.email ?? "" })}
                </Text>
              </Dialog.Body>
              <Dialog.Footer>
                <HStack justify="space-between" w="full">
                  <Button variant="ghost" onClick={() => setRevoking(null)}>
                    {t("common.cancel")}
                  </Button>
                  <Button
                    colorPalette="red"
                    onClick={onRevokeConfirmed}
                    loading={revoke.isPending}
                  >
                    {t("warehouses.revokeAccess")}
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

function Field({
  label,
  value,
  mono,
}: {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
}) {
  return (
    <Box>
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Box fontFamily={mono ? "mono" : undefined}>{value}</Box>
    </Box>
  );
}
