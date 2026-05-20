import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../components/PageHeader";
import RouteTabs from "../components/RouteTabs";

export default function Inventory() {
  const { t } = useTranslation();
  const location = useLocation();
  const tabs = [
    { value: "medicines", to: "/inventory/medicines", label: t("inventory.tabs.medicines") },
    { value: "suppliers", to: "/inventory/suppliers", label: t("inventory.tabs.suppliers") },
    { value: "batches", to: "/inventory/batches", label: t("inventory.tabs.batches") },
    { value: "movements", to: "/inventory/movements", label: t("inventory.tabs.movements") },
    { value: "stocktake", to: "/inventory/stocktake", label: t("inventory.tabs.stocktake") },
  ];
  const activeKey =
    tabs.find((tab) => location.pathname.startsWith(tab.to))?.value ?? "medicines";

  // Stocktake detail (e.g. /inventory/stocktake/<id>) hides the tab strip;
  // it has its own breadcrumb back to the list.
  const isSubpage = /^\/inventory\/stocktake\/[^/]+\/?$/.test(location.pathname);

  return (
    <Box>
      <PageHeader
        breadcrumbs={
          isSubpage
            ? [
                { label: t("inventory.title"), to: "/inventory/medicines" },
                { label: t("inventory.tabs.stocktake"), to: "/inventory/stocktake" },
              ]
            : [{ label: t("inventory.title") }, { label: t(`inventory.tabs.${activeKey}`) }]
        }
        title={t("inventory.title")}
      />
      <Stack gap={4}>
        {!isSubpage && <RouteTabs items={tabs} />}
        <Outlet />
      </Stack>
    </Box>
  );
}
