import {
  Box,
  Flex,
  HStack,
  IconButton,
  Menu,
  Portal,
  Text,
} from "@chakra-ui/react";

import WarehouseSelect from "./WarehouseSelect";
import { useQueryClient } from "@tanstack/react-query";
import { Languages, LogOut, Menu as MenuIcon, Moon, Sun } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useAuth } from "../lib/auth";
import { WAREHOUSE_KEY } from "../lib/transport";
import { useMyWarehousesQuery } from "../queries/warehouses";
import { usePreferencesStore, type Locale } from "../stores/preferences";

export default function TopBar() {
  const { t, i18n } = useTranslation();
  const { user, logout } = useAuth();
  const theme = usePreferencesStore((s) => s.theme);
  const setTheme = usePreferencesStore((s) => s.setTheme);
  const locale = usePreferencesStore((s) => s.locale);
  const setLocale = usePreferencesStore((s) => s.setLocale);
  const toggleSidebar = usePreferencesStore((s) => s.toggleSidebar);

  const flipTheme = () => setTheme(theme === "dark" ? "light" : "dark");

  const flipLocale = () => {
    const next: Locale = locale === "id" ? "en" : "id";
    setLocale(next);
    void i18n.changeLanguage(next);
  };

  const initials = (user?.name || user?.email || "?")
    .split(/[\s@]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((s) => s[0]?.toUpperCase())
    .join("");

  return (
    <Box
      as="header"
      position="sticky"
      top={0}
      zIndex={10}
      bg="bg"
      borderBottomWidth="1px"
      h="56px"
    >
      <Flex align="center" justify="space-between" h="100%" px={4}>
        <HStack gap={2}>
          <IconButton
            aria-label="toggle sidebar"
            variant="ghost"
            size="sm"
            onClick={toggleSidebar}
            display={{ base: "inline-flex", md: "none" }}
          >
            <MenuIcon size={18} />
          </IconButton>
        </HStack>

        <HStack gap={1}>
          {user && <WarehouseSelector />}
          <IconButton aria-label="language" variant="ghost" size="sm" onClick={flipLocale}>
            <HStack gap={1}>
              <Languages size={16} />
              <Text fontSize="xs" fontWeight="medium">
                {locale.toUpperCase()}
              </Text>
            </HStack>
          </IconButton>
          <IconButton aria-label="toggle theme" variant="ghost" size="sm" onClick={flipTheme}>
            {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
          </IconButton>
          {user && (
            <Menu.Root>
              <Menu.Trigger asChild>
                <IconButton aria-label="user menu" variant="ghost" size="sm">
                  <Box
                    colorPalette="blue"
                    bg="colorPalette.solid"
                    color="colorPalette.contrast"
                    w="28px"
                    h="28px"
                    borderRadius="full"
                    display="flex"
                    alignItems="center"
                    justifyContent="center"
                    fontSize="xs"
                    fontWeight="semibold"
                  >
                    {initials}
                  </Box>
                </IconButton>
              </Menu.Trigger>
              <Portal>
                <Menu.Positioner>
                  <Menu.Content>
                    <Menu.Item value="email" disabled>
                      <Text fontSize="sm" color="fg.muted">
                        {user.email}
                      </Text>
                    </Menu.Item>
                    <Menu.Separator />
                    <Menu.Item value="signout" onClick={logout}>
                      <HStack gap={2}>
                        <LogOut size={14} />
                        <Text fontSize="sm">{t("nav.signOut")}</Text>
                      </HStack>
                    </Menu.Item>
                  </Menu.Content>
                </Menu.Positioner>
              </Portal>
            </Menu.Root>
          )}
        </HStack>
      </Flex>
    </Box>
  );
}

function WarehouseSelector() {
  const queryClient = useQueryClient();
  const myWarehousesQ = useMyWarehousesQuery();
  const [current, setCurrent] = useState<string>(() => localStorage.getItem(WAREHOUSE_KEY) || "");

  // Once memberships load, default to the persisted choice or the user's
  // default warehouse.
  useEffect(() => {
    const data = myWarehousesQ.data;
    if (!data || data.warehouses.length === 0) return;
    const persisted = localStorage.getItem(WAREHOUSE_KEY);
    if (persisted && data.warehouses.some((w) => w.id === persisted)) {
      setCurrent(persisted);
      return;
    }
    const def = data.memberships.find((m) => m.isDefault);
    const fallback = def?.warehouseId ?? data.warehouses[0].id;
    setCurrent(fallback);
    localStorage.setItem(WAREHOUSE_KEY, fallback);
  }, [myWarehousesQ.data]);

  if (!myWarehousesQ.data || myWarehousesQ.data.warehouses.length <= 1) return null;

  return (
    <WarehouseSelect
      size="sm"
      width="180px"
      value={current}
      onChange={(v) => {
        setCurrent(v);
        localStorage.setItem(WAREHOUSE_KEY, v);
        // Refetch all warehouse-scoped data with the new X-Warehouse-Id header
        // (the transport reads localStorage per request) — no full page reload.
        void queryClient.invalidateQueries();
      }}
      warehouses={myWarehousesQ.data.warehouses}
    />
  );
}
