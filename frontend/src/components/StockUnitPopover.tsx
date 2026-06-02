import {
  Button,
  Popover,
  Portal,
  RadioGroup,
  Stack,
  Text,
} from "@chakra-ui/react";
import { Ruler } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { StockUnitGroup, StockUnitsByBase } from "../lib/stockUnit";

// StockUnitPopover — toolbar button that opens a popover with one radio group
// per base unit. Each group: "Base" (raw count) + each derivative the catalog
// or page rows know about. The selection is per-base and persisted in
// usePreferencesStore.
type Props = {
  byBase: StockUnitsByBase;
  onChangeBase: (baseName: string, deriv: string) => void;
  groups: StockUnitGroup[];
};

export default function StockUnitPopover({ byBase, onChangeBase, groups }: Props) {
  const { t } = useTranslation();
  const activeCount = groups.filter((g) => byBase[g.baseName]).length;
  return (
    <Popover.Root positioning={{ placement: "bottom-end" }}>
      <Popover.Trigger asChild>
        <Button size="sm" variant="outline">
          <Ruler size={14} />
          {t("inventory.medicines.units")}
          {activeCount > 0 && (
            <Text as="span" color="fg.muted" fontSize="xs" ms={1}>
              · {activeCount}
            </Text>
          )}
        </Button>
      </Popover.Trigger>
      <Portal>
        <Popover.Positioner>
          <Popover.Content minW="240px">
            <Popover.Body>
              <Stack gap={4}>
                {groups.length === 0 && (
                  <Text fontSize="sm" color="fg.muted">
                    {t("inventory.medicines.unitsEmpty")}
                  </Text>
                )}
                {groups.map((g) => (
                  <Stack key={g.baseName} gap={1}>
                    <Text
                      fontSize="xs"
                      color="fg.muted"
                      fontWeight="medium"
                      textTransform="uppercase"
                      letterSpacing="wider"
                    >
                      {g.baseName}
                    </Text>
                    <RadioGroup.Root
                      value={byBase[g.baseName] ?? ""}
                      onValueChange={(d) => onChangeBase(g.baseName, d.value ?? "")}
                    >
                      <Stack gap={2} ps={1}>
                        <RadioGroup.Item value="">
                          <RadioGroup.ItemHiddenInput />
                          <RadioGroup.ItemIndicator />
                          <RadioGroup.ItemText>
                            {t("inventory.medicines.unitBase")}
                          </RadioGroup.ItemText>
                        </RadioGroup.Item>
                        {g.derivatives.map((d) => (
                          <RadioGroup.Item key={d} value={d}>
                            <RadioGroup.ItemHiddenInput />
                            <RadioGroup.ItemIndicator />
                            <RadioGroup.ItemText>{d}</RadioGroup.ItemText>
                          </RadioGroup.Item>
                        ))}
                      </Stack>
                    </RadioGroup.Root>
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
