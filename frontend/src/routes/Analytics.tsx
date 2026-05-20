import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../components/PageHeader";
import RouteTabs from "../components/RouteTabs";

export default function Analytics() {
  const { t } = useTranslation();
  const location = useLocation();
  const tabs = [
    { value: "sales", to: "/analytics/sales", label: t("analytics.tabs.sales") },
    { value: "inventory", to: "/analytics/inventory", label: t("analytics.tabs.inventory") },
    { value: "margins", to: "/analytics/margins", label: t("analytics.tabs.margins") },
  ];
  const activeKey =
    tabs.find((tab) => location.pathname.startsWith(tab.to))?.value ?? "sales";

  return (
    <Box>
      <PageHeader
        breadcrumbs={[
          { label: t("analytics.title") },
          { label: t(`analytics.tabs.${activeKey}`) },
        ]}
        title={t("analytics.title")}
      />
      <Stack gap={4}>
        <RouteTabs items={tabs} />
        <Outlet />
      </Stack>
    </Box>
  );
}
