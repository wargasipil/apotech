import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../components/PageHeader";
import RouteTabs from "../components/RouteTabs";

export default function Settings() {
  const { t } = useTranslation();
  const location = useLocation();
  const tabs = [
    { value: "general", to: "/settings/general", label: t("settings.tabs.general") },
    { value: "units", to: "/settings/units", label: t("settings.tabs.units") },
    { value: "backups", to: "/settings/backups", label: t("settings.tabs.backups") },
  ];
  const activeKey =
    tabs.find((tab) => location.pathname.startsWith(tab.to))?.value ?? "general";

  return (
    <Box>
      <PageHeader
        breadcrumbs={[
          { label: t("nav.settings") },
          { label: t(`settings.tabs.${activeKey}`) },
        ]}
        title={t("settings.title")}
      />
      <Stack gap={4}>
        <RouteTabs items={tabs} />
        <Outlet />
      </Stack>
    </Box>
  );
}
