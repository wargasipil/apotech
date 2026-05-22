import { expect, test } from "./_helpers";

// Multi-warehouse UI. Backend coverage (per-warehouse stock, FEFO, transfers)
// lives in backend/e2e/{warehouse,transfer}_test.go; these specs verify the UI
// wiring: the admin page, the transfers tab, and the POS warehouse gate.
test.describe("warehouses", () => {
  test("admin: add a warehouse", async ({ page }) => {
    await page.goto("/warehouses");
    const code = `WT${Date.now() % 1000000}`;
    await page.getByRole("button", { name: "Add" }).click();
    const drawer = page.getByRole("dialog");
    await drawer.locator("input").nth(0).fill(code);
    await drawer.locator("input").nth(1).fill("Test gudang");
    await drawer.getByRole("button", { name: "Save" }).click();
    await expect(drawer).toBeHidden();
    await expect(page.getByRole("cell", { name: code })).toBeVisible();
  });

  test("transfers tab renders and opens the create drawer", async ({ page }) => {
    await page.goto("/inventory/transfers");
    const newBtn = page.getByRole("button", { name: /New transfer/i });
    await expect(newBtn).toBeVisible();
    await newBtn.click();
    const drawer = page.getByRole("dialog");
    await expect(drawer).toBeVisible();
    await expect(drawer.getByText(/From warehouse/i)).toBeVisible();
    await expect(drawer.getByText(/To warehouse/i)).toBeVisible();
  });

  test("POS asks which warehouse when none is chosen", async ({ page }) => {
    // Ensure the owner has at least two warehouses so the gate is shown.
    await page.goto("/warehouses");
    await page.getByRole("button", { name: "Add" }).click();
    const drawer = page.getByRole("dialog");
    const code = `WG${Date.now() % 1000000}`;
    await drawer.locator("input").nth(0).fill(code);
    await drawer.locator("input").nth(1).fill("Gate test gudang");
    await drawer.getByRole("button", { name: "Save" }).click();
    await expect(drawer).toBeHidden();

    // Clear the active warehouse, then open POS -> the gate must appear.
    await page.evaluate(() => localStorage.removeItem("apotech_warehouse_id"));
    await page.goto("/pos");
    await expect(page.getByText("Select a warehouse")).toBeVisible();

    // Pick the MAIN warehouse; the cart (search box) then loads.
    await page.getByRole("button", { name: /MAIN/ }).click();
    await expect(page.getByPlaceholder(/Search medicine/i)).toBeVisible();
  });
});
