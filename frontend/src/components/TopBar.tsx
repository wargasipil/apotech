import {
  Box,
  Flex,
  HStack,
  IconButton,
  Menu,
  Portal,
  Text,
} from "@chakra-ui/react";

import EnumSelect from "./EnumSelect";
import { Languages, LogOut, Menu as MenuIcon, Moon, Sun } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useAuth } from "../lib/auth";
import { BRANCH_KEY } from "../lib/transport";
import { useMyBranchesQuery } from "../queries/branches";
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
          {user && <BranchSelector />}
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

function BranchSelector() {
  const myBranchesQ = useMyBranchesQuery();
  const [current, setCurrent] = useState<string>(() => localStorage.getItem(BRANCH_KEY) || "");

  // Once memberships load, default to the persisted choice or the user's
  // default branch.
  useEffect(() => {
    const data = myBranchesQ.data;
    if (!data || data.branches.length === 0) return;
    const persisted = localStorage.getItem(BRANCH_KEY);
    if (persisted && data.branches.some((b) => b.id === persisted)) {
      setCurrent(persisted);
      return;
    }
    const def = data.memberships.find((m) => m.isDefault);
    const fallback = def?.branchId ?? data.branches[0].id;
    setCurrent(fallback);
    localStorage.setItem(BRANCH_KEY, fallback);
  }, [myBranchesQ.data]);

  if (!myBranchesQ.data || myBranchesQ.data.branches.length <= 1) return null;

  return (
    <EnumSelect
      size="sm"
      width="180px"
      value={current}
      onChange={(v) => {
        setCurrent(v);
        localStorage.setItem(BRANCH_KEY, v);
        // Hard reload to refetch all branch-scoped data with the new header.
        window.location.reload();
      }}
      items={myBranchesQ.data.branches}
      itemToString={(b) => `${b.code} · ${b.name}`}
      itemToValue={(b) => b.id}
    />
  );
}
