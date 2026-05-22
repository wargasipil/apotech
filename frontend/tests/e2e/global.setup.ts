import { test as setup } from "@playwright/test";

import { loginAs, OWNER } from "./_helpers";

// One-time auth: log in via the UI and persist the resulting localStorage
// (access + refresh tokens) to a JSON file. Every test project loads this
// file via `storageState`, so each spec starts pre-authenticated without
// calling the live Login RPC. Net effect: one login per `make test-browser`
// invocation instead of one per test (which would trip the backend's
// per-email rate limiter at ~5 attempts/minute).
export const STORAGE_STATE = "tests/e2e/.auth/owner.json";

setup("authenticate owner", async ({ page }) => {
  await loginAs(page, OWNER);
  // Pre-select the default warehouse so the POS warehouse gate is satisfied for
  // specs that drive POS directly (the gate only prompts when nothing is chosen).
  await page.evaluate(async () => {
    const token = localStorage.getItem("apotech_access_token");
    const res = await fetch(
      "/api/warehouse_iface.v1.WarehouseService/ListUserWarehouses",
      {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ userId: "" }),
      },
    );
    const data = await res.json();
    const def = (data.memberships || []).find((m: { isDefault?: boolean }) => m.isDefault);
    const id =
      def?.warehouseId ?? (data.warehouses && data.warehouses[0] && data.warehouses[0].id);
    if (id) localStorage.setItem("apotech_warehouse_id", id);
  });
  await page.context().storageState({ path: STORAGE_STATE });
});
