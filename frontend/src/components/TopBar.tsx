import {
  Box,
  Flex,
  HStack,
  IconButton,
  Menu,
  Portal,
  Text,
} from "@chakra-ui/react";
import { Languages, LogOut, Menu as MenuIcon, Moon, Sun } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useAuth } from "../lib/auth";
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
