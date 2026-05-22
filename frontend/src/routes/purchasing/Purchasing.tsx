import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../../components/PageHeader";
import RouteTabs from "../../components/RouteTabs";

// One route per PO status (so the tabs deep-link), plus a "Pemasok" (suppliers
// ledger) tab. The detail + create pages are subpages that hide the tab strip.
const TAB_PATHS = [
  "all",
  "draft",
  "sent",
  "partial",
  "received",
  "closed",
  "voided",
  "suppliers",
];

export default function Purchasing() {
  const { t } = useTranslation();
  const location = useLocation();

  // Subpage = /purchasing/new or /purchasing/<id> — anything single-segment
  // that isn't one of the known tab routes (which must keep showing the tabs).
  const seg = location.pathname.replace(/^\/purchasing\/?/, "").replace(/\/$/, "");
  const isSubpage = seg === "new" || (seg !== "" && !TAB_PATHS.includes(seg));

  const tabs = [
    { value: "all", to: "/purchasing/all", label: t("purchasing.statusTabs.all") },
    { value: "draft", to: "/purchasing/draft", label: t("purchasing.states.draft") },
    { value: "sent", to: "/purchasing/sent", label: t("purchasing.states.sent") },
    { value: "partial", to: "/purchasing/partial", label: t("purchasing.states.partiallyReceived") },
    { value: "received", to: "/purchasing/received", label: t("purchasing.states.received") },
    { value: "closed", to: "/purchasing/closed", label: t("purchasing.states.closed") },
    { value: "voided", to: "/purchasing/voided", label: t("purchasing.states.voided") },
    { value: "suppliers", to: "/purchasing/suppliers", label: t("purchasing.tabs.suppliersLedger") },
  ];

  return (
    <Box>
      <PageHeader
        breadcrumbs={
          isSubpage
            ? [{ label: t("purchasing.title"), to: "/purchasing" }]
            : [{ label: t("purchasing.title") }]
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
