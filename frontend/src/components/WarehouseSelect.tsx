import { useState } from "react";
import { Button, Dialog, Flex, IconButton, Input, Portal, Stack, Text } from "@chakra-ui/react";
import { Check, ChevronDown, Warehouse as WarehouseIcon, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Warehouse } from "../gen/warehouse_iface/v1/warehouse_pb";

type Props = {
  warehouses: readonly Warehouse[];
  value: string;
  onChange: (id: string) => void;
  placeholder?: string;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
  excludeId?: string;
  disabled?: boolean;
};

// Reusable warehouse picker rendered as a searchable modal popup: a button shows
// the current warehouse; clicking opens a centered dialog with a search box + a
// clickable list (mirrors the POS customer picker). Warehouses are a small list,
// so filtering is client-side (no Search RPC). `excludeId` hides one option
// (e.g. the Transfer "To" hides the chosen "From").
export default function WarehouseSelect({
  warehouses,
  value,
  onChange,
  placeholder,
  size = "sm",
  width = "100%",
  excludeId,
  disabled,
}: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");

  const options = excludeId ? warehouses.filter((w) => w.id !== excludeId) : warehouses;
  const selected = warehouses.find((w) => w.id === value);
  const title = placeholder ?? t("common.selectWarehouse");
  const label = selected ? `${selected.code} · ${selected.name}` : title;

  const needle = q.trim().toLowerCase();
  const filtered = needle
    ? options.filter((w) => `${w.code} ${w.name}`.toLowerCase().includes(needle))
    : options;

  const close = () => {
    setOpen(false);
    setQ("");
  };

  return (
    <>
      <Button
        type="button"
        variant="outline"
        size={size}
        width={width}
        disabled={disabled}
        justifyContent="space-between"
        fontWeight="normal"
        onClick={() => setOpen(true)}
      >
        <Flex align="center" gap={2} minW={0}>
          <WarehouseIcon size={14} />
          <Text truncate color={selected ? "fg" : "fg.muted"}>
            {label}
          </Text>
        </Flex>
        <ChevronDown size={16} />
      </Button>

      <Dialog.Root open={open} onOpenChange={(d) => !d.open && close()}>
        <Portal>
          <Dialog.Backdrop />
          <Dialog.Positioner>
            <Dialog.Content>
              <Dialog.Header>
                <Dialog.Title>{title}</Dialog.Title>
                <Dialog.CloseTrigger asChild>
                  <IconButton aria-label="close" variant="ghost" size="sm">
                    <X size={16} />
                  </IconButton>
                </Dialog.CloseTrigger>
              </Dialog.Header>
              <Dialog.Body>
                <Stack gap={3}>
                  <Input
                    placeholder={t("common.search")}
                    value={q}
                    onChange={(e) => setQ(e.target.value)}
                    autoFocus
                  />
                  <Stack gap={1} maxH="320px" overflowY="auto">
                    {filtered.map((w) => (
                      <Flex
                        key={w.id}
                        px={3}
                        py={2}
                        borderRadius="md"
                        _hover={{ bg: "bg.muted" }}
                        cursor="pointer"
                        justify="space-between"
                        align="center"
                        bg={w.id === value ? "bg.muted" : undefined}
                        onClick={() => {
                          onChange(w.id);
                          close();
                        }}
                      >
                        <Text fontSize="sm">
                          {w.code} · {w.name}
                        </Text>
                        {w.id === value && <Check size={14} />}
                      </Flex>
                    ))}
                    {filtered.length === 0 && (
                      <Text color="fg.muted" fontSize="sm" textAlign="center" py={4}>
                        {t("common.noResults")}
                      </Text>
                    )}
                  </Stack>
                </Stack>
              </Dialog.Body>
            </Dialog.Content>
          </Dialog.Positioner>
        </Portal>
      </Dialog.Root>
    </>
  );
}
