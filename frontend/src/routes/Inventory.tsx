import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../components/PageHeader";

// Thin layout for the Inventaris sub-pages. Sub-navigation now lives in the
// sidebar (the expandable "Inventaris" group), so there is no in-page tab
// strip — this just provides a consistent PageHeader + breadcrumb. Medicines
// moved out to the top-level /medicines route.
export default function Inventory() {
  const { t } = useTranslation();
  const location = useLocation();
  const sections = [
    { value: "suppliers", to: "/inventory/suppliers" },
    { value: "batches", to: "/inventory/batches" },
    { value: "movements", to: "/inventory/movements" },
    { value: "stocktake", to: "/inventory/stocktake" },
    { value: "transfers", to: "/inventory/transfers" },
  ];
  const activeKey =
    sections.find((s) => location.pathname.startsWith(s.to))?.value ?? "suppliers";

  // Stocktake detail (e.g. /inventory/stocktake/<id>) shows its own breadcrumb
  // back to the list.
  const isSubpage = /^\/inventory\/stocktake\/[^/]+\/?$/.test(location.pathname);

  return (
    <Box>
      <PageHeader
        breadcrumbs={
          isSubpage
            ? [
                { label: t("inventory.title"), to: "/inventory/suppliers" },
                { label: t("inventory.tabs.stocktake"), to: "/inventory/stocktake" },
              ]
            : [{ label: t("inventory.title") }, { label: t(`inventory.tabs.${activeKey}`) }]
        }
        title={t("inventory.title")}
      />
      <Stack gap={4}>
        <Outlet />
      </Stack>
    </Box>
  );
}
