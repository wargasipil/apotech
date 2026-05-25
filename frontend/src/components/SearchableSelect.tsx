import {
  Combobox,
  HStack,
  Portal,
  Spinner,
  Text,
  createListCollection,
  useFilter,
  useListCollection,
} from "@chakra-ui/react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

// Search-capable select for the apotech UI. Wraps Chakra v3's `Combobox`.
//
// Two operating modes:
//
// 1. **`items` (sync, preload)** — pass the full list of options; the wrapper
//    runs a case-insensitive `useFilter` over them as the user types. Use
//    ONLY for genuinely small static lists (branches, fixed enums covered
//    by <EnumSelect>). Do NOT use for dynamic data — it preloads everything.
//
// 2. **`loadOptions` (async, server-side)** — pass an async callback that
//    returns the matching slice for a query string. The wrapper debounces
//    keystrokes by 250ms, calls `loadOptions(query)`, and renders the
//    result. This is the **hard-rule mode for dynamic data** (medicines,
//    customers, suppliers, batches, prescriptions, sales, POs). See the
//    "Selects" rule in CLAUDE.md.
//
// `selectedLabel` is recommended in async mode for edit drawers that mount
// with a pre-set value before any search has fired — pass the label the
// caller already has (e.g. `customer.name`), and it shows in the trigger
// immediately instead of the raw UUID.
//
// For short fixed-enum option sets (status filters, role pickers, payment
// source, date-range presets), use <EnumSelect> instead — no search input.
export type SearchableSelectProps<T> = {
  value: string | null;
  onChange: (value: string) => void;
  /** Fires with the full picked item (async or sync) alongside onChange. */
  onSelectItem?: (item: T | undefined) => void;

  /** Sync mode: full list of options (small/static lists only). */
  items?: readonly T[];
  /** Async mode: server-side search callback (debounced 250ms). */
  loadOptions?: (query: string) => Promise<readonly T[]>;

  itemToString: (item: T) => string;
  itemToValue: (item: T) => string;

  /** Trigger display when `value` is set but the matching item isn't (yet)
   * in the collection — typical for edit drawers in async mode. */
  selectedLabel?: string;

  placeholder?: string;
  emptyText?: string;
  loadingText?: string;
  disabled?: boolean;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
};

type Entry = { label: string; value: string };

const DEBOUNCE_MS = 250;

export default function SearchableSelect<T>({
  value,
  onChange,
  onSelectItem,
  items,
  loadOptions,
  itemToString,
  itemToValue,
  selectedLabel,
  placeholder,
  emptyText = "No matches",
  loadingText = "Loading…",
  disabled,
  size = "md",
  width,
}: SearchableSelectProps<T>) {
  // Cache labels seen so far so the trigger can keep showing the right name
  // even after the collection rotates (e.g. user types, list changes, but the
  // currently-selected item is no longer in view).
  const labelCacheRef = useRef<Map<string, string>>(new Map());
  // Keep the original items by value so onSelectItem can hand back the full T.
  const itemsByValueRef = useRef<Map<string, T>>(new Map());
  const rememberLabel = useCallback((entry: Entry) => {
    labelCacheRef.current.set(entry.value, entry.label);
  }, []);

  if (selectedLabel && value) {
    // Seed the cache from the caller-provided label so initial render is correct.
    labelCacheRef.current.set(value, selectedLabel);
  }

  const isAsync = !!loadOptions;
  const { contains } = useFilter({ sensitivity: "base" });

  // -------------------- Sync mode -----------------------------------------
  const syncEntries = useMemo<Entry[]>(() => {
    if (isAsync) return [];
    return (items ?? []).map((item) => ({
      label: itemToString(item),
      value: itemToValue(item),
    }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isAsync, items]);

  // -------------------- Async mode ----------------------------------------
  // Track in-flight searches so a slow response for an old query can't
  // overwrite a fresh one. Compare against `latestQueryRef` at resolve time.
  const [asyncEntries, setAsyncEntries] = useState<Entry[]>([]);
  const [isFetching, setIsFetching] = useState(false);
  const latestQueryRef = useRef<string>("");
  const [inputValue, setInputValue] = useState("");

  // Hold the latest itemToString / itemToValue in refs so `runSearch` keeps a
  // stable identity across renders — without this, callers passing inline
  // arrows (the common pattern) would re-create `runSearch` every render,
  // which would in turn cancel the debounce timer before it ever fires.
  const itemToStringRef = useRef(itemToString);
  const itemToValueRef = useRef(itemToValue);
  itemToStringRef.current = itemToString;
  itemToValueRef.current = itemToValue;

  const runSearch = useCallback(
    async (query: string) => {
      if (!loadOptions) return;
      latestQueryRef.current = query;
      setIsFetching(true);
      try {
        const rows = await loadOptions(query);
        // Stale-result guard: only commit if the query hasn't moved on.
        if (latestQueryRef.current !== query) return;
        const mapped = rows.map((item) => ({
          label: itemToStringRef.current(item),
          value: itemToValueRef.current(item),
        }));
        mapped.forEach(rememberLabel);
        rows.forEach((item) => itemsByValueRef.current.set(itemToValueRef.current(item), item));
        setAsyncEntries(mapped);
      } finally {
        if (latestQueryRef.current === query) {
          setIsFetching(false);
        }
      }
    },
    [loadOptions, rememberLabel],
  );

  // Debounced fetch on input change (async mode only).
  useEffect(() => {
    if (!isAsync) return;
    const handle = setTimeout(() => {
      void runSearch(inputValue);
    }, DEBOUNCE_MS);
    return () => clearTimeout(handle);
  }, [inputValue, isAsync, runSearch]);

  // -------------------- Collection ----------------------------------------
  // Cache labels from sync entries so the trigger display also works in sync mode.
  syncEntries.forEach(rememberLabel);
  if (!isAsync) {
    (items ?? []).forEach((item) => itemsByValueRef.current.set(itemToValue(item), item));
  }

  // Build the final entry list. If the current value isn't in the active
  // entries, inject it as a stub so Chakra's Combobox shows the right label
  // in the trigger.
  const effectiveEntries = useMemo<Entry[]>(() => {
    const base = isAsync ? asyncEntries : syncEntries;
    if (!value || base.some((e) => e.value === value)) return base;
    const label = labelCacheRef.current.get(value) ?? selectedLabel ?? value;
    return [{ label, value }, ...base];
  }, [isAsync, asyncEntries, syncEntries, value, selectedLabel]);

  const filterFn = useCallback(
    (itemText: string, filterText: string) => {
      // In async mode the backend already filtered; never re-filter client-side.
      if (isAsync) return true;
      return contains(itemText, filterText);
    },
    [isAsync, contains],
  );

  const { collection, filter, set } = useListCollection<Entry>({
    initialItems: effectiveEntries,
    filter: filterFn,
    itemToString: (i) => i.label,
    itemToValue: (i) => i.value,
  });

  useEffect(() => {
    set(effectiveEntries);
  }, [effectiveEntries, set]);

  const valueArr = value ? [value] : [];

  // Initial fetch when async-mode popover opens for the first time.
  const hasFetchedRef = useRef(false);
  const handleOpenChange = useCallback(
    (d: { open: boolean }) => {
      if (!isAsync || !d.open) return;
      if (!hasFetchedRef.current) {
        hasFetchedRef.current = true;
        void runSearch("");
      }
    },
    [isAsync, runSearch],
  );

  return (
    <Combobox.Root
      collection={collection}
      value={valueArr}
      onValueChange={(d) => {
        const next = d.value[0] ?? "";
        // Cache the picked item's label so it survives subsequent list rotations.
        const picked = collection.items.find((e) => e.value === next);
        if (picked) rememberLabel(picked);
        onChange(next);
        if (onSelectItem) onSelectItem(itemsByValueRef.current.get(next));
      }}
      onInputValueChange={(d) => {
        setInputValue(d.inputValue);
        if (!isAsync) filter(d.inputValue);
      }}
      onOpenChange={handleOpenChange}
      openOnClick
      disabled={disabled}
      size={size}
      width={width}
      selectionBehavior="replace"
    >
      <Combobox.Control>
        <Combobox.Input placeholder={placeholder} />
        <Combobox.IndicatorGroup>
          <Combobox.ClearTrigger />
          <Combobox.Trigger />
        </Combobox.IndicatorGroup>
      </Combobox.Control>
      <Portal>
        <Combobox.Positioner>
          <Combobox.Content>
            {isAsync && isFetching && collection.items.length === 0 && (
              <HStack gap={2} px={3} py={2}>
                <Spinner size="xs" />
                <Text fontSize="sm" color="fg.muted">
                  {loadingText}
                </Text>
              </HStack>
            )}
            <Combobox.Empty>{emptyText}</Combobox.Empty>
            {collection.items.map((item) => (
              <Combobox.Item item={item} key={item.value}>
                <Combobox.ItemText>{item.label}</Combobox.ItemText>
                <Combobox.ItemIndicator />
              </Combobox.Item>
            ))}
          </Combobox.Content>
        </Combobox.Positioner>
      </Portal>
    </Combobox.Root>
  );
}

// Convenience export: build a Combobox-compatible collection from any item
// type. Rarely needed — pass `items` or `loadOptions` to the wrapper instead.
export function buildCollection<T>(
  items: readonly T[],
  itemToString: (i: T) => string,
  itemToValue: (i: T) => string,
) {
  return createListCollection({
    items: items.map((i) => ({ label: itemToString(i), value: itemToValue(i) })),
    itemToString: (i) => i.label,
    itemToValue: (i) => i.value,
  });
}
