import React from "react";
import ReactDOM from "react-dom/client";
import { ChakraProvider, defaultSystem } from "@chakra-ui/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { createBrowserRouter, RouterProvider, Navigate } from "react-router-dom";

import "./lib/i18n";

import App from "./App";
import ErrorBoundary from "./components/ErrorBoundary";
import { AuthProvider } from "./lib/auth";
import { queryClient } from "./lib/queryClient";
import { AppToaster } from "./lib/toaster";
import "./stores/preferences"; // applies persisted theme to <html> on boot

import ProtectedRoute from "./components/ProtectedRoute";
import Login from "./routes/Login";
import Dashboard from "./routes/Dashboard";
import Users from "./routes/Users";
import Customers from "./routes/Customers";
import Inventory from "./routes/Inventory";
import Pos from "./routes/Pos";
import Medicines from "./routes/inventory/Medicines";
import Suppliers from "./routes/inventory/Suppliers";
import Batches from "./routes/inventory/Batches";
import Movements from "./routes/inventory/Movements";
import { Role } from "./gen/auth_iface/v1/policy_pb";

const router = createBrowserRouter([
  {
    path: "/",
    element: <App />,
    children: [
      { path: "login", element: <Login /> },
      {
        element: <ProtectedRoute />,
        children: [
          { index: true, element: <Dashboard /> },
          { path: "pos", element: <Pos /> },
        ],
      },
      {
        element: <ProtectedRoute requiredRole={Role.OWNER} />,
        children: [{ path: "users", element: <Users /> }],
      },
      {
        element: <ProtectedRoute requiredRoles={[Role.OWNER, Role.PHARMACIST, Role.CASHIER]} />,
        children: [{ path: "customers", element: <Customers /> }],
      },
      {
        element: <ProtectedRoute requiredRoles={[Role.OWNER, Role.PHARMACIST]} />,
        children: [
          {
            path: "inventory",
            element: <Inventory />,
            children: [
              { index: true, element: <Navigate to="medicines" replace /> },
              { path: "medicines", element: <Medicines /> },
              { path: "suppliers", element: <Suppliers /> },
              { path: "batches", element: <Batches /> },
              { path: "movements", element: <Movements /> },
            ],
          },
        ],
      },
    ],
  },
]);

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <ChakraProvider value={defaultSystem}>
          <AuthProvider>
            <RouterProvider router={router} />
            <AppToaster />
          </AuthProvider>
          <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-right" />
        </ChakraProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  </React.StrictMode>,
);
