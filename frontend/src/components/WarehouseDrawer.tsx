import { Button, Flex, HStack, Input, Stack, Text } from "@chakra-ui/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import EntityDrawer from "./EntityDrawer";
import type { Warehouse } from "../gen/warehouse_iface/v1/warehouse_pb";
import { toast } from "../lib/toaster";
import {
  useCreateWarehouseMutation,
  useUpdateWarehouseMutation,
} from "../queries/warehouses";

// Shared create/edit drawer for warehouses. Used by both /warehouses (list)
// and /warehouses/:id (detail) so the form lives in one place.
export default function WarehouseDrawer({
  open,
  warehouse,
  onClose,
}: {
  open: boolean;
  warehouse?: Warehouse | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const isEdit = !!warehouse;
  const create = useCreateWarehouseMutation();
  const update = useUpdateWarehouseMutation();
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [phone, setPhone] = useState("");

  // Prefill when the drawer opens for an existing warehouse.
  const [seededId, setSeededId] = useState<string | null>(null);
  if (open && warehouse && seededId !== warehouse.id) {
    setSeededId(warehouse.id);
    setCode(warehouse.code);
    setName(warehouse.name);
    setAddress(warehouse.address);
    setPhone(warehouse.phone);
  }
  if (!open && seededId !== null) setSeededId(null);

  const submit = async () => {
    try {
      if (isEdit && warehouse) {
        await update.mutateAsync({ id: warehouse.id, name, address, phone });
        toast.success(t("common.save") + " ✓");
      } else {
        await create.mutateAsync({ code, name, address, phone });
        toast.success(t("common.create") + " ✓");
        setCode("");
      }
      setName("");
      setAddress("");
      setPhone("");
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={isEdit ? t("warehouses.editTitle") : t("warehouses.addTitle")}
      footer={
        <HStack justify="space-between" w="100%">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={submit}
            loading={create.isPending || update.isPending}
            disabled={!code || !name}
          >
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <Stack gap={3}>
        <Field label={t("warehouses.code")} required>
          <Input value={code} onChange={(e) => setCode(e.target.value)} disabled={isEdit} />
        </Field>
        <Field label={t("warehouses.name")} required>
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label={t("warehouses.address")}>
          <Input value={address} onChange={(e) => setAddress(e.target.value)} />
        </Field>
        <Field label={t("warehouses.phone")}>
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
