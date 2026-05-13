import { Outlet, useLocation } from "react-router-dom";
import { Box } from "@chakra-ui/react";

import AppShell from "./components/AppShell";

// App is the top-level router element. Routes that should render under the
// AppShell wrap themselves with <AppShell><Outlet/></AppShell> indirectly via
// this component. Login is rendered bare (no shell, no auth context required).
export default function App() {
  const location = useLocation();
  const isLogin = location.pathname.startsWith("/login");

  if (isLogin) {
    return (
      <Box minH="100vh" bg="bg">
        <Outlet />
      </Box>
    );
  }

  return <AppShell />;
}
