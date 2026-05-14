import { Box, HStack, IconButton, Stack, Text } from "@chakra-ui/react";
import {
  BarChart3,
  ChevronsLeft,
  ChevronsRight,
  LayoutDashboard,
  LogOut,
  Package,
  Pill,
  ShoppingCart,
  UserRound,
  Users as UsersIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";

import { Role } from "../gen/auth_iface/v1/policy_pb";
import { useAuth } from "../lib/auth";
import { usePreferencesStore } from "../stores/preferences";

type NavItem = {
  to: string;
  label: string;
  icon: typeof Pill;
  roles?: Role[];
};

function buildItems(t: (k: string) => string): NavItem[] {
  return [
    { to: "/", label: t("nav.dashboard"), icon: LayoutDashboard },
    { to: "/pos", label: t("nav.pos"), icon: ShoppingCart },
    {
      to: "/inventory",
      label: t("nav.inventory"),
      icon: Package,
      roles: [Role.OWNER, Role.PHARMACIST],
    },
    {
      to: "/customers",
      label: t("nav.customers"),
      icon: UserRound,
    },
    {
      to: "/analytics",
      label: t("nav.analytics"),
      icon: BarChart3,
      roles: [Role.OWNER, Role.PHARMACIST],
    },
    { to: "/users", label: t("nav.users"), icon: UsersIcon, roles: [Role.OWNER] },
  ];
}

export default function Sidebar() {
  const { t } = useTranslation();
  const collapsed = usePreferencesStore((s) => s.sidebarCollapsed);
  const toggle = usePreferencesStore((s) => s.toggleSidebar);
  const { user, logout } = useAuth();

  const items = buildItems(t).filter(
    (item) => !item.roles || (user && item.roles.includes(user.role)),
  );

  const width = collapsed ? "64px" : "240px";

  return (
    <Box
      as="aside"
      colorPalette="blue"
      width={width}
      bg="bg"
      borderRightWidth="1px"
      height="100vh"
      position="fixed"
      left={0}
      top={0}
      transition="width 150ms ease-out"
      display="flex"
      flexDirection="column"
    >
      {/* Brand */}
      <HStack gap={2} px={4} h="56px" borderBottomWidth="1px">
        <Box color="colorPalette.solid">
          <Pill size={22} />
        </Box>
        {!collapsed && (
          <Text fontWeight="semibold" color="colorPalette.solid" fontSize="lg">
            {t("app.name")}
          </Text>
        )}
      </HStack>

      {/* Nav items */}
      <Stack gap={1} px={2} py={3} flex="1" overflowY="auto">
        {items.map((item) => (
          <NavItemLink key={item.to} item={item} collapsed={collapsed} />
        ))}
      </Stack>

      {/* Footer: user + sign out + collapse */}
      <Stack gap={1} px={2} py={3} borderTopWidth="1px">
        {user && !collapsed && (
          <Box px={2} pb={2}>
            <Text fontSize="sm" fontWeight="medium">
              {user.name || user.email}
            </Text>
            <Text fontSize="xs" color="fg.muted">
              {user.email}
            </Text>
          </Box>
        )}
        {user && (
          <HStack
            as="button"
            gap={3}
            px={3}
            py={2}
            borderRadius="md"
            color="fg.muted"
            _hover={{ bg: "bg.muted" }}
            onClick={logout}
            cursor="pointer"
          >
            <LogOut size={18} />
            {!collapsed && <Text fontSize="sm">{t("nav.signOut")}</Text>}
          </HStack>
        )}
        <IconButton
          aria-label="toggle sidebar"
          variant="ghost"
          size="sm"
          onClick={toggle}
          alignSelf={collapsed ? "center" : "flex-end"}
        >
          {collapsed ? <ChevronsRight size={16} /> : <ChevronsLeft size={16} />}
        </IconButton>
      </Stack>
    </Box>
  );
}

function NavItemLink({ item, collapsed }: { item: NavItem; collapsed: boolean }) {
  const Icon = item.icon;
  return (
    <NavLink to={item.to} end={item.to === "/"}>
      {({ isActive }) => (
        <HStack
          gap={3}
          px={3}
          py={2}
          borderRadius="md"
          bg={isActive ? "bg.muted" : "transparent"}
          color={isActive ? "colorPalette.solid" : "fg"}
          borderLeftWidth={isActive ? "3px" : "0px"}
          borderLeftColor="colorPalette.solid"
          _hover={{ bg: "bg.muted" }}
          title={collapsed ? item.label : undefined}
        >
          <Icon size={18} />
          {!collapsed && <Text fontSize="sm">{item.label}</Text>}
        </HStack>
      )}
    </NavLink>
  );
}
