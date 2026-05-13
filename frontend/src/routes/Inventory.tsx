import { Box, HStack, Link as ChakraLink, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { NavLink, Outlet, useLocation } from "react-router-dom";

import PageHeader from "../components/PageHeader";

export default function Inventory() {
  const { t } = useTranslation();
  const location = useLocation();
  const tabs = [
    { to: "/inventory/medicines", key: "medicines" },
    { to: "/inventory/suppliers", key: "suppliers" },
    { to: "/inventory/batches", key: "batches" },
    { to: "/inventory/movements", key: "movements" },
  ];

  const activeKey =
    tabs.find((tab) => location.pathname.startsWith(tab.to))?.key ?? "medicines";

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("inventory.title") }, { label: t(`inventory.tabs.${activeKey}`) }]}
        title={t("inventory.title")}
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
                {t(`inventory.tabs.${tab.key}`)}
              </NavLink>
            </ChakraLink>
          ))}
        </HStack>
        <Outlet />
      </Stack>
    </Box>
  );
}
