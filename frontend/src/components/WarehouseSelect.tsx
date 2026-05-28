import { useEffect, useRef, useState } from "react";
import {
  Button,
  Dialog,
  Flex,
  IconButton,
  Input,
  Portal,
  Spinner,
  Stack,
  Text,
} from "@chakra-ui/react";
import { Check, ChevronDown, Warehouse as WarehouseIcon, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Warehouse } from "../gen/warehouse_iface/v1/warehouse_pb";

type Props = {
  warehouses?: readonly Warehouse[];
  value: string;
  onChange: (id: string) => void;
  placeholder?: string;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
  excludeId?: string;
  disabled?: boolean;
  // Async mode: when set, the picker ignores `warehouses` as the searchable
  // source and instead debounces input → calls loadOptions(query) → renders the
  // result. Used by the TopBar so the search is backend-driven.
  loadOptions?: (query: string) => Promise<Warehouse[]>;
  // Fallback chip label when the selected value isn't in the currently-loaded
  // list (e.g. the user typed a query that filters it out). Used in async mode.
  selectedLabel?: string;
};

// Reusable warehouse picker rendered as a searchable modal popup: a button
// shows the current warehouse; clicking opens a centered dialog with a search
// box + a clickable list. Two modes:
//   - Static (default): pass `warehouses`; the popover filters them in-memory.
//   - Async: pass `loadOptions(query)`; the popover debounces input + refetches.
// `excludeId` hides one option (e.g. the Transfer "To" hides the chosen "From").
export default function WarehouseSelect({
  warehouses,
  value,
  onChange,
  placeholder,
  size = "sm",
  width = "100%",
  excludeId,
  disabled,
  loadOptions,
  selectedLabel,
}: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");
  const [asyncItems, setAsyncItems] = useState<Warehouse[]>([]);
  const [isFetching, setIsFetching] = useState(false);
  const latestQueryRef = useRef("");

  const isAsync = !!loadOptions;
  const sourceList: readonly Warehouse[] = isAsync ? asyncItems : warehouses ?? [];
  const options = excludeId ? sourceList.filter((w) => w.id !== excludeId) : sourceList;

  // For the chip label: prefer the matching row from the source list; fall back
  // to selectedLabel (async mode where the typed query filters out the picked
  // warehouse); fall back to the placeholder.
  const selectedRow = sourceList.find((w) => w.id === value);
  const title = placeholder ?? t("common.selectWarehouse");
  const label = selectedRow
    ? `${selectedRow.code} · ${selectedRow.name}`
    : selectedLabel ?? title;

  // Async-mode debounce: each keystroke fires loadOptions(q) after 250ms; a
  // ref-based guard discards stale responses if a newer query is in flight.
  useEffect(() => {
    if (!loadOptions || !open) return;
    const trimmed = q.trim();
    latestQueryRef.current = trimmed;
    const handle = setTimeout(async () => {
      setIsFetching(true);
      try {
        const items = await loadOptions(trimmed);
        if (latestQueryRef.current === trimmed) {
          setAsyncItems(items);
        }
      } finally {
        if (latestQueryRef.current === trimmed) setIsFetching(false);
      }
    }, q === "" ? 0 : 250);
    return () => clearTimeout(handle);
  }, [q, loadOptions, open]);

  // Static-mode filter (no-op in async mode — asyncItems already filtered).
  const needle = q.trim().toLowerCase();
  const filtered =
    !isAsync && needle
      ? options.filter((w) => `${w.code} ${w.name}`.toLowerCase().includes(needle))
      : options;

  const close = () => {
    setOpen(false);
    setQ("");
    if (isAsync) setAsyncItems([]);
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
          <Text truncate color={selectedRow || selectedLabel ? "fg" : "fg.muted"}>
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
                  <Flex align="center" gap={2}>
                    <Input
                      placeholder={t("common.search")}
                      value={q}
                      onChange={(e) => setQ(e.target.value)}
                      autoFocus
                    />
                    {isAsync && isFetching && <Spinner size="xs" />}
                  </Flex>
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
                    {filtered.length === 0 && !isFetching && (
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
