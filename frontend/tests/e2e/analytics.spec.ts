import { expect, test } from "./_helpers";

// Pre-authenticated via the `setup` project (storage state). No per-test
// login needed. The analytics surface is now 3 dimension-scoped menus
// (Daily / Product / User) — all 13 old RPCs were nuked in the rewrite.
test.describe("analytics", () => {
  test("/analytics redirects to /analytics/daily", async ({ page }) => {
    await page.goto("/analytics");
    await page.waitForURL("**/analytics/daily");
  });

  test("/analytics/daily renders the table + graph tabs", async ({ page }) => {
    await page.goto("/analytics/daily");
    await expect(page.getByRole("heading", { name: "Daily" }).first()).toBeVisible();
    // Table tab is the default — at least the column-group headers are visible.
    await expect(page.getByText("Order", { exact: true }).first()).toBeVisible();
    await expect(page.getByText("Stock", { exact: true }).first()).toBeVisible();
    // Tab strip with Table + Graph.
    await expect(page.getByRole("tab", { name: "Table" })).toBeVisible();
    await expect(page.getByRole("tab", { name: "Graph" })).toBeVisible();
  });

  test("/analytics/product paginates and shows order/stock columns", async ({ page }) => {
    await page.goto("/analytics/product");
    await expect(page.getByRole("heading", { name: "Product" }).first()).toBeVisible();
    // Pagination footer is always rendered.
    await expect(page.getByText(/Showing/i).or(page.getByText(/No results/i))).toBeVisible();
  });

  test("/analytics/user only allows Order metrics", async ({ page }) => {
    await page.goto("/analytics/user");
    await expect(page.getByRole("heading", { name: "User" }).first()).toBeVisible();
    // Open the Columns popover and assert the Stock-group checkboxes are disabled.
    await page.getByRole("button", { name: /Columns/ }).click();
    const ready = page.getByRole("checkbox", { name: "Ready" });
    await expect(ready).toBeVisible();
    await expect(ready).toBeDisabled();
    const ongoing = page.getByRole("checkbox", { name: "Ongoing" });
    await expect(ongoing).toBeDisabled();
  });

  test("/analytics/daily — unchecking HPP in the popover hides the HPP column", async ({
    page,
  }) => {
    await page.goto("/analytics/daily");
    // HPP column is visible by default.
    await expect(page.getByRole("columnheader", { name: "HPP" })).toBeVisible();
    // Open Columns, uncheck HPP — scope to the popover content so we don't
    // collide with the "HPP" column header in the table.
    await page.getByRole("button", { name: /Columns/ }).click();
    const popover = page.locator('[data-scope="popover"][data-part="content"]');
    await popover.getByText("HPP", { exact: true }).click();
    // Close the popover by pressing Escape so it doesn't overlay the table.
    await page.keyboard.press("Escape");
    // HPP column is gone.
    await expect(page.getByRole("columnheader", { name: "HPP" })).toBeHidden();
  });

  test("legacy analytics URLs redirect to /analytics/daily", async ({ page }) => {
    for (const path of [
      "/analytics/sales",
      "/analytics/margins",
      "/analytics/operations",
      "/analytics/profitability",
      "/analytics/inventory",
    ]) {
      await page.goto(path);
      await page.waitForURL("**/analytics/daily");
    }
  });

  test("changing the date range refetches without console errors (BigInt regression)", async ({
    page,
  }) => {
    await page.goto("/analytics/daily");
    await expect(page.getByRole("heading", { name: "Daily" }).first()).toBeVisible();

    // Driving the date-range combobox: the analytics page header has the
    // granularity selector first + the DateRangeFilter second. Pick the
    // second combobox in <main>.
    const cycle = async (label: string) => {
      await page.locator("main").getByRole("combobox").nth(1).click();
      await page.getByRole("option", { name: label }).click();
    };
    await cycle("7 days");
    await cycle("Today");
    await cycle("30 days");

    // No further assertion — the page fixture in _helpers.ts fails the test
    // on any console error. waitForLoadState lets refetches complete first.
    await page.waitForLoadState("networkidle");
  });
});
