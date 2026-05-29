import { Box, HStack, Heading, Stack } from "@chakra-ui/react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import ColumnsPopover from "../../components/ColumnsPopover";
import DateRangeFilter, {
  resolveRange,
  type DateRange,
} from "../../components/DateRangeFilter";
import MetricTable from "../../components/MetricTable";
import Pagination from "../../components/Pagination";
import { type Sort } from "../../gen/analytics_iface/v1/analytics_pb";
import {
  DEFAULT_USER_FIELDS,
  fieldsToMetricTypes,
} from "../../lib/analyticsFields";
import { usePageState } from "../../lib/pagination";
import { useUserMetricQuery } from "../../queries/analytics";
import { useUserRefs } from "../../queries/refs";
import { clearSortIfHidden, defaultGroups } from "./Daily";

// User menu — one row per cashier user. Order metrics only (STOCK rejected by
// the backend on this dimension). Paginated. Names resolved via useUserRefs.
export default function User() {
  const { t } = useTranslation();
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const [visibleFields, setVisibleFields] = useState<Set<string>>(
    () => new Set(DEFAULT_USER_FIELDS),
  );
  const [sort, setSort] = useState<Sort | undefined>(undefined);

  const metricTypes = useMemo(
    () => fieldsToMetricTypes(visibleFields),
    [visibleFields],
  );

  const visibleKey = useMemo(
    () => [...visibleFields].sort().join(","),
    [visibleFields],
  );
  const resetKey = `${range.preset}|${range.fromUnix}|${range.toUnix}|${visibleKey}|${JSON.stringify(sort ?? null)}`;
  const { page, setPage, pageSize, setPageSize } = usePageState(resetKey);

  const q = useUserMetricQuery({
    metricTypes,
    filter: { fromUnix: BigInt(range.fromUnix), toUnix: BigInt(range.toUnix) },
    sort,
    page,
    pageSize,
  });

  const ids = q.data?.ids ?? [];
  const refs = useUserRefs(ids);
  const labelById = useMemo(() => {
    const m = new Map<string, string>();
    for (const [id, r] of refs.entries()) m.set(id, r.name || r.email);
    return m;
  }, [refs]);

  return (
    <Stack gap={4}>
      <HStack justify="space-between" wrap="wrap" gap={3}>
        <Heading size="sm">{t("analytics.menu.user")}</Heading>
        <HStack gap={2} wrap="wrap">
          <ColumnsPopover
            value={visibleFields}
            onChange={(next) => {
              setVisibleFields(next);
              clearSortIfHidden(sort, next, setSort);
            }}
            groups={defaultGroups(t, {
              disableStock: true,
              disableStockReason: t("analytics.errors.userStockUnsupported"),
            })}
            defaults={DEFAULT_USER_FIELDS}
          />
          <DateRangeFilter value={range} onChange={setRange} />
        </HStack>
      </HStack>

      <Box>
        <MetricTable
          ids={ids}
          order={q.data?.order}
          metricTypes={metricTypes}
          visibleFields={visibleFields}
          labelById={labelById}
          sort={sort}
          onSortChange={setSort}
          dimensionHeader={t("analytics.menu.user")}
          isLoading={q.isLoading}
        />
      </Box>

      <Pagination
        page={page}
        pageSize={pageSize}
        total={q.data?.total ?? 0}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />
    </Stack>
  );
}
