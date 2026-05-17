import { expect, test } from "./_helpers";

// Pre-authenticated via the `setup` project (storage state). No per-test
// login needed.
test.describe("analytics", () => {

  test("/analytics/sales renders the revenue trend + hour heatmap", async ({ page }) => {
    await page.goto("/analytics/sales");
    await expect(page.getByRole("heading", { name: "Revenue trend" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Sales by hour of day" })).toBeVisible();
    // Heatmap renders the day-of-week labels.
    for (const day of ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]) {
      await expect(page.getByText(day, { exact: true })).toBeVisible();
    }
  });

  test("/analytics/inventory renders the expiry tiles + turnover table", async ({ page }) => {
    await page.goto("/analytics/inventory");
    await expect(page.getByText("Expiring in 30d")).toBeVisible();
    await expect(page.getByText("Expiring in 90d")).toBeVisible();
    await expect(page.getByText("Expiring in 180d")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Turnover (period)" })).toBeVisible();
  });

  test("/analytics/margins renders the top-margin table", async ({ page }) => {
    await page.goto("/analytics/margins");
    await expect(page.getByRole("heading", { name: "Top margin" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Margin per medicine" })).toBeVisible();
  });

  test("changing the date range refetches without console errors (BigInt regression)", async ({
    page,
  }) => {
    await page.goto("/analytics/sales");
    await expect(page.getByRole("heading", { name: "Revenue trend" })).toBeVisible();

    // This combobox is the DateRangeFilter — its onChange sends new
    // fromUnix/toUnix BigInt values into queryKey. Before the queryKeyHashFn
    // fix in lib/queryClient.ts, TanStack Query would throw "Do not know how
    // to serialize a BigInt" the moment we touched this control.
    const range = page.getByRole("combobox").first();
    await range.selectOption("7d");
    await range.selectOption("today");
    await range.selectOption("30d");

    // No assertion needed — the page fixture in _helpers.ts fails any test
    // that produced a console error. If the BigInt bug ever returns, this
    // test goes red. The waitForLoadState is just to make sure refetches had
    // a chance to fire before the fixture's teardown checks errors.
    await page.waitForLoadState("networkidle");
  });
});
