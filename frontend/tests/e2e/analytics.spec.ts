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

    // The DateRangeFilter is now a Chakra Select widget (button-based combobox,
    // not a real <select>). Drive it via click + option text. The reason the
    // test exists at all: this control's onChange sends new fromUnix/toUnix
    // BigInt values into queryKey. Before the queryKeyHashFn fix in
    // lib/queryClient.ts, TanStack Query would throw "Do not know how to
    // serialize a BigInt" the moment we touched this control.
    const cycle = async (label: string) => {
      // First combobox on the page is the date-range picker.
      await page.getByRole("combobox").first().click();
      await page.getByRole("option", { name: label }).click();
    };
    await cycle("7 days");
    await cycle("Today");
    await cycle("30 days");

    // No further assertion needed — the page fixture in _helpers.ts fails
    // any test that produced a console error. If the BigInt bug ever
    // returns, this test goes red. waitForLoadState lets refetches finish
    // before the fixture's teardown checks errors.
    await page.waitForLoadState("networkidle");
  });
});
