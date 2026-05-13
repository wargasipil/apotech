import { Box, Flex } from "@chakra-ui/react";
import { Outlet } from "react-router-dom";

import Sidebar from "./Sidebar";
import TopBar from "./TopBar";
import { usePreferencesStore } from "../stores/preferences";

export default function AppShell() {
  const collapsed = usePreferencesStore((s) => s.sidebarCollapsed);
  const sidebarWidth = collapsed ? "64px" : "240px";

  return (
    <Flex direction="row" minH="100vh">
      <Sidebar />
      <Box
        flex="1"
        ml={sidebarWidth}
        transition="margin-left 150ms ease-out"
        display="flex"
        flexDirection="column"
      >
        <TopBar />
        <Box as="main" flex="1" px={6} py={4}>
          <Outlet />
        </Box>
      </Box>
    </Flex>
  );
}
