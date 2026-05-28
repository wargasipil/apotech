import { Badge } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

const MS_PER_DAY = 86_400_000;

// Color-coded days-to-expiry badge: ≤30d danger, ≤90d warning, else success;
// already-expired shows the localized "expired" label in red. The label is a
// relative phrase ("today" / "in 5 days" / "in 4 months" / "in over a year")
// driven by i18n with i18next plural support.
export default function ExpiryBadge({ expiry }: { expiry: string }) {
  const { t } = useTranslation();
  const days = Math.ceil((new Date(expiry).getTime() - Date.now()) / MS_PER_DAY);

  const color: "red" | "orange" | "green" =
    days <= 0 || days <= 30 ? "red" : days <= 90 ? "orange" : "green";

  return <Badge colorPalette={color}>{label(t, days)}</Badge>;
}

function label(t: (k: string, opts?: Record<string, unknown>) => string, days: number): string {
  if (days <= 0) return t("expiryBadge.expired");
  if (days === 0) return t("expiryBadge.today");
  if (days === 1) return t("expiryBadge.tomorrow");
  if (days < 30) return t("expiryBadge.inDays", { count: days });
  if (days < 90) return t("expiryBadge.inWeeks", { count: Math.round(days / 7) });
  if (days < 365) return t("expiryBadge.inMonths", { count: Math.round(days / 30) });
  return t("expiryBadge.inOverYear");
}
