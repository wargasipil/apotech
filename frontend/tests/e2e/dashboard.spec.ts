import { expect, test } from "./_helpers";

// Role-aware Dashboard. The storage-state fixture authenticates as OWNER, so
// we only cover the OWNER branch here. PHARMACIST + CASHIER variants are a
// follow-up once the suite grows a non-owner login fixture (documented gap).
test.describe("dashboard", () => {
  test("OWNER sees the Business health section + 6 tiles + 7-day trend", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /Business health/i })).toBeVisible();

    // Six labeled tiles (Revenue / Profit / Sales / Items / Avg basket / Low stock / Expiring).
    // We assert via the i18n labels — exact text matches the en locale.
    for (const label of [
      "Revenue today",
      "Profit today",
      "Sales today",
      "Items sold today",
      "Avg basket",
      "Low stock",
      "Expiring in 30 days",
    ]) {
      await expect(page.getByText(label, { exact: true })).toBeVisible();
    }

    // 7-day trend chart heading is rendered.
    await expect(page.getByRole("heading", { name: /Last 7 days/i })).toBeVisible();
  });

  test("OWNER tile click navigates to the linked detail page", async ({ page }) => {
    await page.goto("/");
    // The Revenue tile links to /orders. The label sits inside a RouterLink-wrapped
    // tile; clicking the label text element follows the link.
    await page.getByText("Revenue today", { exact: true }).click();
    await page.waitForURL("**/orders");
    await expect(page).toHaveURL(/\/orders$/);
  });
});
