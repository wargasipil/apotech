import { Badge } from "@chakra-ui/react";

const MS_PER_DAY = 86_400_000;

// Color-coded days-to-expiry badge: ≤30d danger, ≤90d warning, else success;
// already-expired shows the localized "expired" label in red.
export default function ExpiryBadge({
  expiry,
  expiredLabel,
}: {
  expiry: string;
  expiredLabel: string;
}) {
  const days = Math.ceil((new Date(expiry).getTime() - Date.now()) / MS_PER_DAY);
  if (days <= 0) return <Badge colorPalette="red">{expiredLabel}</Badge>;
  if (days <= 30) return <Badge colorPalette="red">{days}d</Badge>;
  if (days <= 90) return <Badge colorPalette="orange">{days}d</Badge>;
  return <Badge colorPalette="green">{days}d</Badge>;
}
