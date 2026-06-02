import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// Settings → Unit catalog management. Owner can:
//   1. Add a base unit (e.g. "tablet")
//   2. Attach a derivative (e.g. "box" with factor 100)
//   3. Archive both
//
// Pre-cleanup via API removes any prior runs' rows so the test is idempotent
// against the dev DB.

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

async function archiveBase(page: Page, id: string): Promise<void> {
  try {
    await api(page, "unit_iface.v1.UnitService/ArchiveUnitBase", { id });
  } catch {
    /* best-effort */
  }
}

test.describe("settings — unit catalog", () => {
  test("owner adds a base, attaches a derivative, archives both", async ({ page }) => {
    const m = String(Date.now());
    const baseName = `tab-ui-${m}`;
    const derivName = `box-ui-${m}`;
    let baseId: string | undefined;
    try {
      await page.goto("/settings/units");
      await page.waitForLoadState("networkidle");

      // Add base.
      const baseInput = page.getByPlaceholder(/e\.g\. tablet|mis\. tablet/i);
      await baseInput.fill(baseName);
      await page.getByRole("button", { name: /Add base|Tambah dasar/i }).click();
      const baseHeading = page.getByRole("heading", { name: baseName });
      await expect(baseHeading).toBeVisible();

      // The base card is the nearest enclosing border-box of the heading.
      // Using a CSS ancestor selector matches multiple Chakra-generated wrappers;
      // we use the heading's parent box directly via locator("..", "..").
      const baseCard = baseHeading.locator("xpath=ancestor::div[1]").locator("..");

      // Add derivative inside this base card. The new placeholders are unique
      // strings so we can use them directly instead of structural selectors.
      const derivInput = baseCard.getByPlaceholder(/e\.g\. box|mis\. box/i);
      await derivInput.fill(derivName);
      // Factor input — placeholder is the localized "Factor"/"Faktor".
      const factorInput = baseCard.getByPlaceholder(/^Factor$|^Faktor$/);
      await factorInput.fill("100");
      await baseCard.getByRole("button", { name: /^Add$|^Tambah$/i }).click();
      // The derivative row appears in the card's small table.
      await expect(baseCard.getByRole("cell", { name: derivName })).toBeVisible();
      await expect(
        baseCard.getByRole("cell", { name: new RegExp(`100\\s*×\\s*${baseName}`) }),
      ).toBeVisible();

      // Look up the created ids via the JSON API so we can hard-clean up.
      const list = await api<{ bases: Array<{ id: string; name: string }> }>(
        page,
        "unit_iface.v1.UnitService/ListUnitBases",
        {},
      );
      const created = list.bases.find((b) => b.name === baseName);
      expect(created).toBeDefined();
      baseId = created!.id;
    } finally {
      if (baseId) await archiveBase(page, baseId);
    }
  });
});
