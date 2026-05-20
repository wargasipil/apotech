import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../../components/PageHeader";
import RouteTabs from "../../components/RouteTabs";

export default function Purchasing() {
  const { t } = useTranslation();
  const location = useLocation();

  // Hide the tabs when viewing a single PO or the create-form (they have
  // their own breadcrumbs and don't belong to a tab).
  const isSubpage =
    location.pathname.startsWith("/purchasing/new") ||
    /^\/purchasing\/[^/]+\/?$/.test(location.pathname);

  const tabs = [
    { value: "all", to: "/purchasing/all", label: t("purchasing.tabs.all") },
    { value: "outstanding", to: "/purchasing/outstanding", label: t("purchasing.tabs.outstanding") },
    { value: "suppliersLedger", to: "/purchasing/suppliers", label: t("purchasing.tabs.suppliersLedger") },
  ];
  const activeKey =
    tabs.find((tab) => location.pathname.startsWith(tab.to))?.value ?? "all";

  return (
    <Box>
      <PageHeader
        breadcrumbs={
          isSubpage
            ? [{ label: t("purchasing.title"), to: "/purchasing" }]
            : [
                { label: t("purchasing.title") },
                { label: t(`purchasing.tabs.${activeKey}`) },
              ]
        }
        title={t("purchasing.title")}
      />
      <Stack gap={4}>
        {!isSubpage && <RouteTabs items={tabs} />}
        <Outlet />
      </Stack>
    </Box>
  );
}
