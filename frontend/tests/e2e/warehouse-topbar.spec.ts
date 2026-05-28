import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// The TopBar warehouse picker is the standardized searchable popup over
// useMyWarehousesQuery() — client-side substring filter on `code` + `name`,
// case-insensitive. This spec seeds its own warehouse (auto-granted to OWNER
// at CreateWarehouse time), then drives the TopBar UI: open → type → click →
// chip updates. Pins the search behavior with deterministic data so dev-DB
// pollution can't break the assertion.

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

type WarehouseSeed = { id: string; code: string; name: string };

async function seedWarehouse(page: Page, marker: string): Promise<WarehouseSeed> {
  await page.goto("/");
  const code = `WT${marker}`;
  const name = `TopBar test ${marker}`;
  const res = await api<{ warehouse: { id: string } }>(
    page,
    "warehouse_iface.v1.WarehouseService/CreateWarehouse",
    { code, name },
  );
  return { id: res.warehouse.id, code, name };
}

async function archiveWarehouse(page: Page, id: string): Promise<void> {
  try {
    await api(page, "warehouse_iface.v1.WarehouseService/ArchiveWarehouse", { id });
  } catch {
    /* best-effort */
  }
}

test.describe("TopBar warehouse picker", () => {
  test("searching by partial code finds the seeded warehouse and switches active", async ({
    page,
  }) => {
    const marker = String(Date.now() % 1000000);
    const seed = await seedWarehouse(page, marker);
    try {
      // Force a reload so useMyWarehousesQuery() picks up the new grant
      // (CreateWarehouse auto-grants the creator membership).
      await page.goto("/");

      // The TopBar warehouse button shows the current chip. Click it to open
      // the searchable popup.
      const trigger = page.getByRole("button", { name: /Select warehouse|·/i }).first();
      await trigger.click();
      const picker = page.getByRole("dialog");
      await expect(picker).toBeVisible();

      // Type a substring of the new warehouse's code — search is client-side
      // case-insensitive over `code + " " + name`.
      await picker.getByRole("textbox").fill(seed.code.toLowerCase());

      // The matching row should appear and be clickable.
      const row = picker.getByText(`${seed.code} · ${seed.name}`);
      await expect(row).toBeVisible();
      await row.click();

      // Picker closes; the TopBar chip now shows the new warehouse.
      await expect(picker).not.toBeVisible();
      await expect(page.getByText(`${seed.code} · ${seed.name}`)).toBeVisible();
    } finally {
      await archiveWarehouse(page, seed.id);
    }
  });
});
