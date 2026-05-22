import { HStack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import DatePickerField from "./DatePicker";
import EnumSelect from "./EnumSelect";

export type RangePreset = "today" | "7d" | "30d" | "90d" | "ytd" | "custom";

export type DateRange = {
  preset: RangePreset;
  fromUnix: number;
  toUnix: number;
  customFrom?: string; // YYYY-MM-DD
  customTo?: string;
};

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

export function resolveRange(preset: RangePreset, customFrom?: string, customTo?: string): DateRange {
  const now = new Date();
  const end = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1); // exclusive end
  switch (preset) {
    case "today": {
      const from = startOfDay(now);
      return { preset, fromUnix: Math.floor(from.getTime() / 1000), toUnix: Math.floor(end.getTime() / 1000) };
    }
    case "7d": {
      const from = new Date(end);
      from.setDate(from.getDate() - 7);
      return { preset, fromUnix: Math.floor(from.getTime() / 1000), toUnix: Math.floor(end.getTime() / 1000) };
    }
    case "30d": {
      const from = new Date(end);
      from.setDate(from.getDate() - 30);
      return { preset, fromUnix: Math.floor(from.getTime() / 1000), toUnix: Math.floor(end.getTime() / 1000) };
    }
    case "90d": {
      const from = new Date(end);
      from.setDate(from.getDate() - 90);
      return { preset, fromUnix: Math.floor(from.getTime() / 1000), toUnix: Math.floor(end.getTime() / 1000) };
    }
    case "ytd": {
      const from = new Date(now.getFullYear(), 0, 1);
      return { preset, fromUnix: Math.floor(from.getTime() / 1000), toUnix: Math.floor(end.getTime() / 1000) };
    }
    case "custom": {
      const f = customFrom ? new Date(customFrom + "T00:00:00") : startOfDay(now);
      const t = customTo ? new Date(customTo + "T23:59:59") : end;
      return {
        preset,
        customFrom,
        customTo,
        fromUnix: Math.floor(f.getTime() / 1000),
        toUnix: Math.floor(t.getTime() / 1000),
      };
    }
  }
}

type Props = {
  value: DateRange;
  onChange: (next: DateRange) => void;
};

type RangeOption = { value: RangePreset; label: string };

export default function DateRangeFilter({ value, onChange }: Props) {
  const { t } = useTranslation();
  const options: RangeOption[] = [
    { value: "today", label: t("analytics.range.today") },
    { value: "7d", label: t("analytics.range.7d") },
    { value: "30d", label: t("analytics.range.30d") },
    { value: "90d", label: t("analytics.range.90d") },
    { value: "ytd", label: t("analytics.range.ytd") },
    { value: "custom", label: t("analytics.range.custom") },
  ];
  return (
    <HStack gap={2}>
      <EnumSelect
        size="sm"
        width="160px"
        value={value.preset}
        onChange={(v) => onChange(resolveRange(v as RangePreset, value.customFrom, value.customTo))}
        items={options}
        itemToString={(o) => o.label}
        itemToValue={(o) => o.value}
      />
      {value.preset === "custom" && (
        <>
          <DatePickerField
            size="sm"
            value={value.customFrom ?? ""}
            onChange={(v) => onChange(resolveRange("custom", v, value.customTo))}
          />
          <DatePickerField
            size="sm"
            value={value.customTo ?? ""}
            onChange={(v) => onChange(resolveRange("custom", value.customFrom, v))}
          />
        </>
      )}
    </HStack>
  );
}
