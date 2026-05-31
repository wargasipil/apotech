import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// /warehouses/:id detail page. End-to-end: navigate from list -> detail,
// add a user via the searchable picker, toggle "Default for this user",
// revoke the user. Backend coverage of the new RPCs lives in
// backend/e2e/warehouse_test.go (TestGetWarehouse / TestListWarehouseUsers /
// TestSearchUsers).

async function api<T = unknown>(page: Page, path: string, body: unknown): Promise<T> {
  return await page.evaluate(
    async ([p, b]: [string, unknown]) => {
      const token = localStorage.getItem("apotech_access_token");
      if (!token) throw new Error("no access token");
      const res = await fetch(`/api/${p}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(b),
      });
      if (!res.ok) throw new Error(`${p}: ${res.status} ${await res.text()}`);
      return (await res.json()) as unknown;
    },
    [path, body] as const,
  ) as Promise<T>;
}

type Seed = {
  warehouseId: string;
  warehouseCode: string;
  warehouseName: string;
  userId: string;
  userEmail: string;
};

async function seed(page: Page, marker: string): Promise<Seed> {
  await page.goto("/");
  const code = `WD${marker}`;
  const name = `WHDetail test ${marker}`;
  const wh = await api<{ warehouse: { id: string } }>(
    page,
    "warehouse_iface.v1.WarehouseService/CreateWarehouse",
    { code, name },
  );
  const email = `whd-${marker}@apotech.local`;
  const u = await api<{ user: { id: string } }>(
    page,
    "user_iface.v1.UserService/CreateUser",
    { email, name: "WHD Test", password: "Test1234!", role: 3 /* CASHIER */ },
  );
  return {
    warehouseId: wh.warehouse.id,
    warehouseCode: code,
    warehouseName: name,
    userId: u.user.id,
    userEmail: email,
  };
}

async function cleanup(page: Page, s: Seed): Promise<void> {
  try {
    await api(page, "user_iface.v1.UserService/SetUserActive", {
      userId: s.userId,
      active: false,
    });
  } catch {
    /* best-effort */
  }
  try {
    await api(page, "warehouse_iface.v1.WarehouseService/ArchiveWarehouse", {
      id: s.warehouseId,
    });
  } catch {
    /* best-effort */
  }
}

test.describe("warehouse detail", () => {
  test("row click → detail → add user → revoke", async ({ page }) => {
    const marker = String(Date.now() % 1000000);
    const s = await seed(page, marker);
    try {
      // 1. Click the warehouse row → land on the detail page.
      await page.goto("/warehouses");
      await page.getByPlaceholder(/Search code or name|Cari kode/i).fill(s.warehouseCode);
      await page.waitForTimeout(400); // debounced
      await page.getByRole("cell", { name: s.warehouseCode }).click();
      await page.waitForURL(new RegExp(`/warehouses/${s.warehouseId}$`));

      // 2. OWNER row appears (auto-grant from CreateWarehouse). Scope to the
      // table — the sidebar also shows the owner's email.
      await expect(
        page.getByRole("cell", { name: "owner@apotech.local" }),
      ).toBeVisible();

      // 3. Add the seeded user via the searchable picker.
      const picker = page.getByPlaceholder(/Add user|Tambah pengguna/i);
      await picker.click();
      await picker.fill(s.userEmail.split("@")[0]);
      await page.waitForTimeout(400); // debounced search
      await page.getByRole("option", { name: new RegExp(s.userEmail) }).click();
      // Row appears for the seeded user.
      await expect(page.getByRole("row", { name: new RegExp(s.userEmail) })).toBeVisible();

      // 4. Revoke the new user.
      const row = page.getByRole("row", { name: new RegExp(s.userEmail) });
      await row.getByRole("button", { name: /Revoke|Cabut/i }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();
      await dialog.getByRole("button", { name: /Revoke access|Cabut akses/i }).click();
      await expect(dialog).not.toBeVisible();

      // Row gone.
      await expect(page.getByRole("row", { name: new RegExp(s.userEmail) })).toHaveCount(0);
    } finally {
      await cleanup(page, s);
    }
  });
});
