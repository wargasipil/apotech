import { expect, test } from "./_helpers";

// Order history list + detail page.
// The "Created by" column resolves cashier_user_id via useUserRefs; rows are
// clickable and navigate to /orders/:id which renders the OrderDetail page
// (BackButton + PageHeader + sections).
test.describe("orders", () => {
  test("list shows the Created by column header", async ({ page }) => {
    await page.goto("/orders");
    await expect(page.getByRole("columnheader", { name: "Created by" })).toBeVisible();
  });

  test("row click navigates to /orders/:id and the detail page renders", async ({ page }) => {
    await page.goto("/orders");
    // Wait for the table body to populate; assume the dev DB has at least one
    // completed sale (the COMPLETED tab is the default).
    const firstRow = page.locator("tbody tr").first();
    await expect(firstRow).toBeVisible();
    await firstRow.click();

    // Land on the detail URL — pattern is /orders/<uuid> (uuid v4 = 8-4-4-4-12 hex).
    await page.waitForURL(/\/orders\/[0-9a-f-]{36}$/);

    // BackButton present (HARD RULE), info section heading visible, "Created by"
    // field label visible on the detail page.
    await expect(page.getByRole("button", { name: /Back/i })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Information" })).toBeVisible();
    await expect(page.getByText("Created by", { exact: true })).toBeVisible();
  });
});
