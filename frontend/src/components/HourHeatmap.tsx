import { Box, Grid, Text } from "@chakra-ui/react";
import { useMemo } from "react";

import type { HeatmapCell } from "../gen/analytics_iface/v1/sales_pb";

const DAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

type Props = {
  cells: HeatmapCell[];
};

export default function HourHeatmap({ cells }: Props) {
  // Build a (day, hour) -> count map and find the max for color scaling.
  const { byKey, max } = useMemo(() => {
    const byKey = new Map<string, number>();
    let max = 0;
    for (const c of cells) {
      const k = `${c.dayOfWeek}:${c.hour}`;
      const n = Number(c.saleCount);
      byKey.set(k, n);
      if (n > max) max = n;
    }
    return { byKey, max };
  }, [cells]);

  return (
    <Box overflowX="auto">
      <Grid templateColumns="48px repeat(24, 1fr)" gap="2px" minW="640px">
        {/* Header row */}
        <Box />
        {Array.from({ length: 24 }, (_, h) => (
          <Box key={`hh-${h}`} fontSize="2xs" color="fg.muted" textAlign="center">
            {h}
          </Box>
        ))}
        {/* Body */}
        {DAY_LABELS.map((label, day) => (
          <Box key={`day-${day}`} display="contents">
            <Text fontSize="xs" color="fg.muted" alignSelf="center">
              {label}
            </Text>
            {Array.from({ length: 24 }, (_, h) => {
              const n = byKey.get(`${day}:${h}`) ?? 0;
              const ratio = max > 0 ? n / max : 0;
              return (
                <Box
                  key={`c-${day}-${h}`}
                  h="24px"
                  borderRadius="sm"
                  bg={ratio === 0 ? "bg.muted" : `blue.${ratio < 0.25 ? 200 : ratio < 0.5 ? 400 : ratio < 0.75 ? 600 : 800}`}
                  title={`${label} ${h}:00 — ${n} sales`}
                />
              );
            })}
          </Box>
        ))}
      </Grid>
    </Box>
  );
}
