import { expect, test } from "./_helpers";

// Multi-warehouse UI. Backend coverage (per-warehouse stock, FEFO, transfers)
// lives in backend/e2e/{warehouse,transfer}_test.go; these specs verify the UI
// wiring: the admin page, the transfers tab, and the POS warehouse gate.
test.describe("warehouses", () => {
  test("admin: add a warehouse", async ({ page }) => {
    await page.goto("/warehouses");
    // List is now paginated server-side (page size 25, sorted by code ASC) and
    // the dev DB carries hundreds of accumulated test warehouses. Use a "0"-
    // prefix code so the new row sorts to page 1 where the assertion lives.
    const code = `0WT${Date.now() % 1000000}`;
    await page.getByRole("button", { name: "Add" }).click();
    const drawer = page.getByRole("dialog");
    await drawer.locator("input").nth(0).fill(code);
    await drawer.locator("input").nth(1).fill("Test gudang");
    await drawer.getByRole("button", { name: "Save" }).click();
    await expect(drawer).toBeHidden();
    await expect(page.getByRole("cell", { name: code })).toBeVisible();
  });

  test("admin: search narrows the list to a single matching code", async ({ page }) => {
    await page.goto("/warehouses");
    // Seed a warehouse with a unique prefix so search has something specific
    // to land on regardless of dev-DB state.
    const code = `SRCH${Date.now() % 1000000}`;
    await page.getByRole("button", { name: "Add" }).click();
    const drawer = page.getByRole("dialog");
    await drawer.locator("input").nth(0).fill(code);
    await drawer.locator("input").nth(1).fill("Searchable gudang");
    await drawer.getByRole("button", { name: "Save" }).click();
    await expect(drawer).toBeHidden();

    // Type the unique prefix; backend SearchWarehouses filters server-side.
    await page.getByPlaceholder(/Search code or name|Cari kode/i).fill(code);
    await page.waitForTimeout(400); // debounced
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

    // The gate's picker is the standardized searchable popup. Open it and
    // pick the warehouse we just seeded (deterministic — doesn't depend on
    // MAIN being present in the dev DB's user_warehouses).
    await page.getByRole("button", { name: /Select warehouse/i }).click();
    const picker = page.getByRole("dialog");
    await picker.getByRole("textbox").fill(code);
    await picker.getByText(`${code} · Gate test gudang`).click();
    await expect(page.getByPlaceholder(/Search medicine/i)).toBeVisible();
  });
});
