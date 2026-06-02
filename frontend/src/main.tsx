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
import Orders from "./routes/Orders";
import OrderDetail from "./routes/OrderDetail";
import Inventory from "./routes/Inventory";
import Pos from "./routes/Pos";
import Medicines from "./routes/inventory/Medicines";
import MedicineDetail from "./routes/inventory/MedicineDetail";
import Suppliers from "./routes/inventory/Suppliers";
import Batches from "./routes/inventory/Batches";
import Movements from "./routes/inventory/Movements";
import Stocktake from "./routes/inventory/Stocktake";
import StocktakeDetail from "./routes/inventory/StocktakeDetail";
import Analytics from "./routes/Analytics";
import DailyAnalytics from "./routes/analytics/Daily";
import ProductAnalytics from "./routes/analytics/Product";
import UserAnalytics from "./routes/analytics/User";
import Prescriptions from "./routes/Prescriptions";
import Purchasing from "./routes/purchasing/Purchasing";
import Tax from "./routes/Tax";
import Bpjs from "./routes/Bpjs";
import Warehouses from "./routes/Warehouses";
import WarehouseDetail from "./routes/WarehouseDetail";
import Settings from "./routes/Settings";
import SettingsGeneral from "./routes/settings/SettingsGeneral";
import SettingsUnits from "./routes/settings/SettingsUnits";
import SettingsBackups from "./routes/settings/SettingsBackups";
import Transfers from "./routes/inventory/Transfers";
import PurchaseOrdersList from "./routes/purchasing/PurchaseOrdersList";
import SuppliersLedger from "./routes/purchasing/SuppliersLedger";
import NewPurchaseOrder from "./routes/purchasing/NewPurchaseOrder";
import PurchaseOrderDetail from "./routes/purchasing/PurchaseOrderDetail";
import { POStatus } from "./gen/purchasing_iface/v1/order_pb";
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
        children: [
          { path: "users", element: <Users /> },
          {
            path: "settings",
            element: <Settings />,
            children: [
              { index: true, element: <Navigate to="general" replace /> },
              { path: "general", element: <SettingsGeneral /> },
              { path: "units", element: <SettingsUnits /> },
              { path: "backups", element: <SettingsBackups /> },
            ],
          },
        ],
      },
      {
        element: <ProtectedRoute requiredRoles={[Role.OWNER, Role.PHARMACIST, Role.CASHIER]} />,
        children: [
          { path: "customers", element: <Customers /> },
          { path: "orders", element: <Orders /> },
          { path: "orders/:id", element: <OrderDetail /> },
        ],
      },
      {
        element: <ProtectedRoute requiredRoles={[Role.OWNER, Role.PHARMACIST]} />,
        children: [
          { path: "medicines", element: <Medicines /> },
          { path: "medicines/:id", element: <MedicineDetail /> },
          {
            path: "inventory",
            element: <Inventory />,
            children: [
              { index: true, element: <Navigate to="suppliers" replace /> },
              // Moved to the top-level /medicines route; keep a redirect for old links.
              { path: "medicines", element: <Navigate to="/medicines" replace /> },
              { path: "suppliers", element: <Suppliers /> },
              { path: "batches", element: <Batches /> },
              { path: "movements", element: <Movements /> },
              { path: "stocktake", element: <Stocktake /> },
              { path: "stocktake/:id", element: <StocktakeDetail /> },
              { path: "transfers", element: <Transfers /> },
            ],
          },
          {
            path: "analytics",
            element: <Analytics />,
            children: [
              { index: true, element: <Navigate to="daily" replace /> },
              { path: "daily", element: <DailyAnalytics /> },
              { path: "product", element: <ProductAnalytics /> },
              { path: "user", element: <UserAnalytics /> },
              // Back-compat: every URL the old analytics shipped at points here now.
              { path: "operations", element: <Navigate to="/analytics/daily" replace /> },
              { path: "profitability", element: <Navigate to="/analytics/daily" replace /> },
              { path: "inventory", element: <Navigate to="/analytics/daily" replace /> },
              { path: "sales", element: <Navigate to="/analytics/daily" replace /> },
              { path: "margins", element: <Navigate to="/analytics/daily" replace /> },
            ],
          },
          {
            path: "purchasing",
            element: <Purchasing />,
            children: [
              { index: true, element: <Navigate to="all" replace /> },
              { path: "all", element: <PurchaseOrdersList /> },
              { path: "draft", element: <PurchaseOrdersList status={POStatus.PO_STATUS_DRAFT} /> },
              { path: "sent", element: <PurchaseOrdersList status={POStatus.PO_STATUS_SENT} /> },
              { path: "partial", element: <PurchaseOrdersList status={POStatus.PO_STATUS_PARTIALLY_RECEIVED} /> },
              { path: "received", element: <PurchaseOrdersList status={POStatus.PO_STATUS_RECEIVED} /> },
              { path: "closed", element: <PurchaseOrdersList status={POStatus.PO_STATUS_CLOSED} /> },
              { path: "voided", element: <PurchaseOrdersList status={POStatus.PO_STATUS_VOIDED} /> },
              { path: "suppliers", element: <SuppliersLedger /> },
              { path: "new", element: <NewPurchaseOrder /> },
              { path: ":id", element: <PurchaseOrderDetail /> },
            ],
          },
          { path: "prescriptions", element: <Prescriptions /> },
          { path: "bpjs", element: <Bpjs /> },
          {
            element: <ProtectedRoute requiredRole={Role.OWNER} />,
            children: [
              { path: "tax", element: <Tax /> },
              { path: "warehouses", element: <Warehouses /> },
              { path: "warehouses/:id", element: <WarehouseDetail /> },
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
