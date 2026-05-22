import { useEffect, useRef, useState } from "react";

// Default page size for every server-paginated list.
export const DEFAULT_PAGE_SIZE = 25;
export const PAGE_SIZE_OPTIONS = [10, 25, 50, 100] as const;

// Limit used by page-level "preload the whole list" fetches — name-lookup
// display maps and the few remaining preload-mode selects. These are exempt
// from the server-side-search hard rule (which governs dynamic <SearchableSelect>
// option sources, not page-level name maps). Denormalizing names into the
// parent response is the eventual fix; until then this caps the preload.
export const ALL_LIMIT = 1000;

// usePageState centralizes list pagination: 0-based page + page size, and
// resets to page 0 whenever the caller's filter signature (resetKey) changes
// so a filtered result never lands the user on an out-of-range page.
export function usePageState(resetKey: string) {
  const [page, setPage] = useState(0);
  const [pageSize, setPageSizeRaw] = useState<number>(DEFAULT_PAGE_SIZE);
  const prev = useRef(resetKey);

  useEffect(() => {
    if (prev.current !== resetKey) {
      prev.current = resetKey;
      setPage(0);
    }
  }, [resetKey]);

  const setPageSize = (n: number) => {
    setPageSizeRaw(n);
    setPage(0);
  };

  return { page, setPage, pageSize, setPageSize };
}
