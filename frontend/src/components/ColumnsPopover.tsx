import { Button, Checkbox, HStack, Popover, Portal, Stack, Text } from "@chakra-ui/react";
import { Columns3, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";

// Per-field column-visibility picker. A single trigger button (with an N/T
// counter) opens a Popover with sectioned checkboxes, one per metric field.
// Scales to any number of metric groups added later: each group becomes a
// section in the popover; no horizontal sprawl.
//
// Convention: field ids are `"<group>.<field>"` strings — e.g. `"order.terjual"`,
// `"stock.ready"`. The caller owns the `Set<string>` state.

export type FieldSpec = {
  id: string;
  label: string;
  disabled?: boolean;
};

export type GroupSpec = {
  id: string;
  label: string;
  fields: FieldSpec[];
  // A whole group can be disabled (e.g. STOCK on the User page); all its
  // checkboxes render disabled + the section header carries the tooltip.
  disabled?: boolean;
  disabledReason?: string;
};

type Props = {
  value: Set<string>;
  onChange: (next: Set<string>) => void;
  groups: GroupSpec[];
  // Defaults the Reset button restores. Defaults to "every non-disabled field
  // selected" when omitted.
  defaults?: Set<string>;
};

export default function ColumnsPopover({ value, onChange, groups, defaults }: Props) {
  const { t } = useTranslation();

  const allFields = groups.flatMap((g) =>
    g.fields.map((f) => ({ ...f, groupDisabled: !!g.disabled })),
  );
  const enabledFields = allFields.filter((f) => !f.disabled && !f.groupDisabled);
  const visibleCount = enabledFields.filter((f) => value.has(f.id)).length;
  const totalCount = enabledFields.length;

  const toggle = (id: string, on: boolean) => {
    const next = new Set(value);
    if (on) next.add(id);
    else next.delete(id);
    onChange(next);
  };

  const reset = () => {
    if (defaults) {
      onChange(new Set(defaults));
      return;
    }
    onChange(new Set(enabledFields.map((f) => f.id)));
  };

  return (
    <Popover.Root positioning={{ placement: "bottom-end" }}>
      <Popover.Trigger asChild>
        <Button size="sm" variant="outline">
          <Columns3 size={14} />
          {t("analytics.columns.button")}
          <Text as="span" color="fg.muted" fontSize="xs" ms={1}>
            {t("analytics.columns.count", { n: visibleCount, total: totalCount })}
          </Text>
        </Button>
      </Popover.Trigger>
      <Portal>
        <Popover.Positioner>
          <Popover.Content minW="240px">
            <Popover.Body>
              <Stack gap={3}>
                <HStack justify="space-between">
                  <Text fontSize="sm" fontWeight="medium">
                    {t("analytics.columns.popoverTitle")}
                  </Text>
                  <Button size="xs" variant="ghost" onClick={reset}>
                    <RotateCcw size={12} />
                    {t("analytics.columns.reset")}
                  </Button>
                </HStack>
                {groups.map((g) => (
                  <Stack key={g.id} gap={1}>
                    <Text
                      fontSize="xs"
                      color={g.disabled ? "fg.subtle" : "fg.muted"}
                      fontWeight="medium"
                      textTransform="uppercase"
                      letterSpacing="wider"
                      title={g.disabled ? g.disabledReason : undefined}
                    >
                      {g.label}
                    </Text>
                    <Stack gap={1} ps={1}>
                      {g.fields.map((f) => {
                        const disabled = !!g.disabled || !!f.disabled;
                        return (
                          <Checkbox.Root
                            key={f.id}
                            checked={value.has(f.id)}
                            onCheckedChange={(d) => toggle(f.id, !!d.checked)}
                            disabled={disabled}
                            title={disabled ? g.disabledReason : undefined}
                            size="sm"
                          >
                            <Checkbox.HiddenInput />
                            <Checkbox.Control />
                            <Checkbox.Label>{f.label}</Checkbox.Label>
                          </Checkbox.Root>
                        );
                      })}
                    </Stack>
                  </Stack>
                ))}
              </Stack>
            </Popover.Body>
          </Popover.Content>
        </Popover.Positioner>
      </Portal>
    </Popover.Root>
  );
}
