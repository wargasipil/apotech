import { Box, Stack, Text } from "@chakra-ui/react";
import type { ReactNode } from "react";
import { Link as RouterLink } from "react-router-dom";

// Shared tile used by the role-keyed Dashboard sections. Clickable when `to`
// is set (wrapped in <RouterLink>). `tone` colors the border + value: default
// neutral, "warning" for soft alerts (e.g. low-stock count > 0), "danger" for
// hard alerts. `hint` renders a small line below the value (e.g. "last sale
// 14:32" under "My last sale").
type Tone = "default" | "warning" | "danger";

type Props = {
  label: string;
  value: string;
  hint?: string;
  to?: string;
  tone?: Tone;
};

export default function DashboardTile({ label, value, hint, to, tone = "default" }: Props) {
  const colors = toneColors(tone);
  const card = (
    <Box
      bg="bg.subtle"
      borderWidth="1px"
      borderColor={colors.border}
      borderRadius="lg"
      p={5}
      transition="background 100ms ease-out"
      _hover={to ? { bg: "bg.muted" } : undefined}
      cursor={to ? "pointer" : "default"}
      h="100%"
    >
      <Stack gap={1}>
        <Text fontSize="sm" color="fg.muted">
          {label}
        </Text>
        <Text fontSize="2xl" fontWeight="semibold" fontFamily="mono" color={colors.value}>
          {value}
        </Text>
        {hint && (
          <Text fontSize="xs" color="fg.muted">
            {hint}
          </Text>
        )}
      </Stack>
    </Box>
  );
  if (to) {
    return (
      <RouterLink to={to} style={{ textDecoration: "none", display: "block", height: "100%" }}>
        {card as ReactNode}
      </RouterLink>
    );
  }
  return card;
}

function toneColors(tone: Tone): { border: string; value: string } {
  switch (tone) {
    case "warning":
      return { border: "orange.300", value: "orange.600" };
    case "danger":
      return { border: "red.300", value: "red.600" };
    default:
      return { border: "border", value: "fg" };
  }
}
