import { Box, HStack, IconButton, Spacer, Stack, Text } from "@chakra-ui/react";
import {
  ArrowLeftRight,
  BarChart3,
  Boxes,
  Building2,
  ChevronDown,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  ClipboardList,
  FileText,
  LayoutDashboard,
  LogOut,
  Package,
  Percent,
  Pill,
  Receipt,
  Repeat,
  ShoppingCart,
  Truck,
  UserRound,
  Users as UsersIcon,
  Warehouse as WarehouseIcon,
} from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { NavLink, useLocation } from "react-router-dom";

import { Role } from "../gen/auth_iface/v1/policy_pb";
import { useAuth } from "../lib/auth";
import { usePreferencesStore } from "../stores/preferences";

type NavLeaf = {
  to: string;
  label: string;
  icon: typeof Pill;
  roles?: Role[];
};

type NavGroup = {
  kind: "group";
  label: string;
  icon: typeof Pill;
  roles?: Role[];
  children: NavLeaf[];
};

type NavEntry = NavLeaf | NavGroup;

function isGroup(e: NavEntry): e is NavGroup {
  return "kind" in e && e.kind === "group";
}

function buildItems(t: (k: string) => string): NavEntry[] {
  return [
    { to: "/", label: t("nav.dashboard"), icon: LayoutDashboard },
    { to: "/pos", label: t("nav.pos"), icon: ShoppingCart },
    {
      to: "/medicines",
      label: t("nav.medicines"),
      icon: Pill,
      roles: [Role.OWNER, Role.PHARMACIST],
    },
    {
      kind: "group",
      label: t("nav.inventory"),
      icon: Package,
      roles: [Role.OWNER, Role.PHARMACIST],
      children: [
        { to: "/purchasing", label: t("nav.purchasing"), icon: Truck },
        { to: "/inventory/suppliers", label: t("inventory.tabs.suppliers"), icon: Building2 },
        { to: "/inventory/batches", label: t("inventory.tabs.batches"), icon: Boxes },
        { to: "/inventory/movements", label: t("inventory.tabs.movements"), icon: ArrowLeftRight },
        { to: "/inventory/stocktake", label: t("inventory.tabs.stocktake"), icon: ClipboardList },
        { to: "/inventory/transfers", label: t("inventory.tabs.transfers"), icon: Repeat },
      ],
    },
    {
      to: "/customers",
      label: t("nav.customers"),
      icon: UserRound,
    },
    {
      to: "/orders",
      label: t("nav.orders"),
      icon: Receipt,
      roles: [Role.OWNER, Role.PHARMACIST, Role.CASHIER],
    },
    {
      to: "/prescriptions",
      label: t("nav.prescriptions"),
      icon: FileText,
      roles: [Role.OWNER, Role.PHARMACIST],
    },
    {
      to: "/analytics",
      label: t("nav.analytics"),
      icon: BarChart3,
      roles: [Role.OWNER, Role.PHARMACIST],
    },
    { to: "/tax", label: t("nav.tax"), icon: Percent, roles: [Role.OWNER] },
    {
      to: "/bpjs",
      label: t("nav.bpjs"),
      icon: FileText,
      roles: [Role.OWNER, Role.PHARMACIST],
    },
    { to: "/warehouses", label: t("nav.warehouses"), icon: WarehouseIcon, roles: [Role.OWNER] },
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
        {items.map((item) =>
          isGroup(item) ? (
            <NavGroupItem key={item.label} group={item} collapsed={collapsed} />
          ) : (
            <NavItemLink key={item.to} item={item} collapsed={collapsed} />
          ),
        )}
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

function NavItemLink({ item, collapsed }: { item: NavLeaf; collapsed: boolean }) {
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

// Expandable sidebar group (e.g. Inventaris). Auto-expands when any child route
// is active. When the rail is collapsed (icon-only) the children render as flat
// icon links so every destination stays reachable.
function NavGroupItem({ group, collapsed }: { group: NavGroup; collapsed: boolean }) {
  const { user } = useAuth();
  const location = useLocation();
  const Icon = group.icon;

  const children = group.children.filter(
    (c) => !c.roles || (user && c.roles.includes(user.role)),
  );
  const anyChildActive = children.some((c) => location.pathname.startsWith(c.to));
  const [open, setOpen] = useState(anyChildActive);

  useEffect(() => {
    if (anyChildActive) setOpen(true);
  }, [anyChildActive]);

  if (collapsed) {
    return (
      <>
        {children.map((c) => (
          <NavItemLink key={c.to} item={c} collapsed />
        ))}
      </>
    );
  }

  return (
    <Box>
      <HStack
        as="button"
        width="100%"
        gap={3}
        px={3}
        py={2}
        borderRadius="md"
        color={anyChildActive ? "colorPalette.solid" : "fg"}
        _hover={{ bg: "bg.muted" }}
        cursor="pointer"
        onClick={() => setOpen((o) => !o)}
      >
        <Icon size={18} />
        <Text fontSize="sm">{group.label}</Text>
        <Spacer />
        {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
      </HStack>
      {open && (
        <Stack gap={1} pl={4} mt={1}>
          {children.map((c) => (
            <NavItemLink key={c.to} item={c} collapsed={false} />
          ))}
        </Stack>
      )}
    </Box>
  );
}
