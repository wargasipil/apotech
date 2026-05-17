import { Box, HStack, Link as ChakraLink, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { NavLink, Outlet, useLocation } from "react-router-dom";

import PageHeader from "../../components/PageHeader";

export default function Purchasing() {
  const { t } = useTranslation();
  const location = useLocation();

  // Hide the tabs when viewing a single PO or the create-form (they have
  // their own breadcrumbs and don't belong to a tab).
  const isSubpage =
    location.pathname.startsWith("/purchasing/new") ||
    /^\/purchasing\/[^/]+\/?$/.test(location.pathname);

  const tabs = [
    { to: "/purchasing/all", key: "all" },
    { to: "/purchasing/outstanding", key: "outstanding" },
    { to: "/purchasing/suppliers", key: "suppliersLedger" },
  ];
  const activeKey =
    tabs.find((tab) => location.pathname.startsWith(tab.to))?.key ?? "all";

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
        {!isSubpage && (
          <HStack gap={4} borderBottomWidth="1px" pb={2}>
            {tabs.map((tab) => (
              <ChakraLink key={tab.to} asChild>
                <NavLink
                  to={tab.to}
                  style={({ isActive }) => ({
                    fontWeight: isActive ? 600 : 400,
                    borderBottom: isActive
                      ? "2px solid currentColor"
                      : "2px solid transparent",
                    paddingBottom: "6px",
                    textDecoration: "none",
                  })}
                >
                  {t(`purchasing.tabs.${tab.key}`)}
                </NavLink>
              </ChakraLink>
            ))}
          </HStack>
        )}
        <Outlet />
      </Stack>
    </Box>
  );
}
