import { Box, HStack, Link as ChakraLink, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { NavLink, Outlet, useLocation } from "react-router-dom";

import PageHeader from "../components/PageHeader";

export default function Analytics() {
  const { t } = useTranslation();
  const location = useLocation();
  const tabs = [
    { to: "/analytics/sales", key: "sales" },
    { to: "/analytics/inventory", key: "inventory" },
    { to: "/analytics/margins", key: "margins" },
  ];
  const activeKey =
    tabs.find((tab) => location.pathname.startsWith(tab.to))?.key ?? "sales";

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
        <HStack gap={4} borderBottomWidth="1px" pb={2}>
          {tabs.map((tab) => (
            <ChakraLink key={tab.to} asChild>
              <NavLink
                to={tab.to}
                style={({ isActive }) => ({
                  fontWeight: isActive ? 600 : 400,
                  borderBottom: isActive ? "2px solid currentColor" : "2px solid transparent",
                  paddingBottom: "6px",
                  textDecoration: "none",
                })}
              >
                {t(`analytics.tabs.${tab.key}`)}
              </NavLink>
            </ChakraLink>
          ))}
        </HStack>
        <Outlet />
      </Stack>
    </Box>
  );
}
