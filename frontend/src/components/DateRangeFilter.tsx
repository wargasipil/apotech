import { HStack, NativeSelect, Input } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

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

export default function DateRangeFilter({ value, onChange }: Props) {
  const { t } = useTranslation();
  return (
    <HStack gap={2}>
      <NativeSelect.Root size="sm" width="auto">
        <NativeSelect.Field
          value={value.preset}
          onChange={(e) => onChange(resolveRange(e.target.value as RangePreset, value.customFrom, value.customTo))}
        >
          <option value="today">{t("analytics.range.today")}</option>
          <option value="7d">{t("analytics.range.7d")}</option>
          <option value="30d">{t("analytics.range.30d")}</option>
          <option value="90d">{t("analytics.range.90d")}</option>
          <option value="ytd">{t("analytics.range.ytd")}</option>
          <option value="custom">{t("analytics.range.custom")}</option>
        </NativeSelect.Field>
        <NativeSelect.Indicator />
      </NativeSelect.Root>
      {value.preset === "custom" && (
        <>
          <Input
            size="sm"
            type="date"
            value={value.customFrom ?? ""}
            onChange={(e) => onChange(resolveRange("custom", e.target.value, value.customTo))}
          />
          <Input
            size="sm"
            type="date"
            value={value.customTo ?? ""}
            onChange={(e) => onChange(resolveRange("custom", value.customFrom, e.target.value))}
          />
        </>
      )}
    </HStack>
  );
}
