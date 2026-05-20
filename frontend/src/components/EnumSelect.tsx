import { Portal, Select, createListCollection } from "@chakra-ui/react";
import { useMemo } from "react";

// Wrapper around Chakra v3's `Select` (popover, no search). Use for short
// fixed-enum option sets — status filters, role pickers, payment source,
// date-range presets, branch switcher. For dynamic / long lists, use
// <SearchableSelect> instead.
//
// Single-select by design; multi-select can land as a follow-up when a
// real call site needs it.
export type EnumSelectProps<T> = {
  value: string | null;
  onChange: (value: string) => void;
  items: readonly T[];
  itemToString: (item: T) => string;
  itemToValue: (item: T) => string;
  placeholder?: string;
  disabled?: boolean;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
};

type Entry = { label: string; value: string };

export default function EnumSelect<T>({
  value,
  onChange,
  items,
  itemToString,
  itemToValue,
  placeholder,
  disabled,
  size = "md",
  width,
}: EnumSelectProps<T>) {
  // Memoize the collection so identity stays stable across renders — Chakra's
  // select machine spins if the collection swaps every paint.
  const collection = useMemo(
    () =>
      createListCollection<Entry>({
        items: items.map((item) => ({
          label: itemToString(item),
          value: itemToValue(item),
        })),
        itemToString: (i) => i.label,
        itemToValue: (i) => i.value,
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [items],
  );

  const valueArr = value ? [value] : [];

  return (
    <Select.Root
      collection={collection}
      value={valueArr}
      onValueChange={(d) => onChange(d.value[0] ?? "")}
      disabled={disabled}
      size={size}
      width={width}
    >
      <Select.HiddenSelect />
      <Select.Control>
        <Select.Trigger>
          <Select.ValueText placeholder={placeholder} />
        </Select.Trigger>
        <Select.IndicatorGroup>
          <Select.Indicator />
        </Select.IndicatorGroup>
      </Select.Control>
      <Portal>
        <Select.Positioner>
          <Select.Content>
            {collection.items.map((item) => (
              <Select.Item item={item} key={item.value}>
                <Select.ItemText>{item.label}</Select.ItemText>
                <Select.ItemIndicator />
              </Select.Item>
            ))}
          </Select.Content>
        </Select.Positioner>
      </Portal>
    </Select.Root>
  );
}
