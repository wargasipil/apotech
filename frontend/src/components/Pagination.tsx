import { Button, HStack, Text } from "@chakra-ui/react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";

import EnumSelect from "./EnumSelect";
import { PAGE_SIZE_OPTIONS } from "../lib/pagination";

// Shared list pagination control: "Showing X–Y of N" + Prev/Next + optional
// page-size selector. Page is 0-based. Render it under any server-paginated
// table; pair it with usePageState (lib/pagination.ts).
export type PaginationProps = {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
  onPageSizeChange?: (size: number) => void;
};

export default function Pagination({
  page,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
}: PaginationProps) {
  const { t } = useTranslation();
  const from = total === 0 ? 0 : page * pageSize + 1;
  const to = Math.min((page + 1) * pageSize, total);
  const canPrev = page > 0;
  const canNext = to < total;

  return (
    <HStack justify="space-between" pt={1} flexWrap="wrap" gap={2}>
      <Text fontSize="sm" color="fg.muted">
        {t("common.pagination.showing", { from, to, total })}
      </Text>
      <HStack gap={2}>
        {onPageSizeChange && (
          <EnumSelect
            size="sm"
            width="32"
            value={String(pageSize)}
            onChange={(v) => onPageSizeChange(Number(v))}
            items={PAGE_SIZE_OPTIONS.map((n) => String(n))}
            itemToString={(s) => t("common.pagination.perPage", { count: Number(s) })}
            itemToValue={(s) => s}
          />
        )}
        <Button
          size="sm"
          variant="outline"
          disabled={!canPrev}
          onClick={() => onPageChange(page - 1)}
        >
          <ChevronLeft size={16} />
          {t("common.pagination.prev")}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={!canNext}
          onClick={() => onPageChange(page + 1)}
        >
          {t("common.pagination.next")}
          <ChevronRight size={16} />
        </Button>
      </HStack>
    </HStack>
  );
}
