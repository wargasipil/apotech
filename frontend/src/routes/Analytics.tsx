import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../components/PageHeader";
import RouteTabs from "../components/RouteTabs";

export default function Analytics() {
  const { t } = useTranslation();
  const location = useLocation();
  const tabs = [
    { value: "daily", to: "/analytics/daily", label: t("analytics.menu.daily") },
    { value: "product", to: "/analytics/product", label: t("analytics.menu.product") },
    { value: "user", to: "/analytics/user", label: t("analytics.menu.user") },
  ];
  const activeKey =
    tabs.find((tab) => location.pathname.startsWith(tab.to))?.value ?? "daily";

  return (
    <Box>
      <PageHeader
        breadcrumbs={[
          { label: t("analytics.title") },
          { label: t(`analytics.menu.${activeKey}`) },
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
