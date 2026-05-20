import { expect, test } from "./_helpers";

// Auth from storageState (loaded by the chromium project). The Go integration
// tests (backend/e2e/stocktake_test.go) cover the data-layer guarantees
// exhaustively (FEFO-adjacent semantics, validation guards, movement chain).
// This spec only verifies the UI wires the RPCs correctly and renders without
// console errors — start a session, see the detail page, void it.
test.describe("stocktake", () => {
  test("New session → detail → Void cleans up", async ({ page }) => {
    // Ensure no stale DRAFT session is blocking — visit list, void any.
    await page.goto("/inventory/stocktake");
    await expect(page.getByRole("heading", { name: "Inventory" })).toBeVisible();
    // Click any visible Draft badge row that has a Void path; simpler: just
    // attempt to create. If the rule trips, we react.

    const initialDraftCount = await page
      .getByRole("cell", { name: /Draft|Draf/i })
      .count();

    // If there's already a DRAFT, the New session button should produce a
    // FailedPrecondition toast — handle that case by navigating into the
    // existing draft and voiding it first.
    if (initialDraftCount > 0) {
      // Open the first DRAFT row.
      await page.getByRole("cell", { name: /Draft|Draf/i }).first().click();
      await page.waitForURL(/\/inventory\/stocktake\/[0-9a-f-]+$/);
      // Void it; detail page shows the action bar disappearing after Void.
      await page.getByRole("button", { name: /^Void$|^Batalkan$/i }).click();
      // The action bar (Complete, Void, Add batches) is hidden once not DRAFT.
      await expect(
        page.getByRole("button", { name: /Complete|Selesaikan/i }),
      ).toBeHidden();
      await page.goto("/inventory/stocktake");
    }

    // Click "New session".
    await page.getByRole("button", { name: /New session|Sesi baru/i }).click();

    // Should navigate to /inventory/stocktake/<id> — detail page renders the
    // session name + a Draft status badge.
    await page.waitForURL(/\/inventory\/stocktake\/[0-9a-f-]+$/);

    // The page header for Inventory keeps the "Stocktake" breadcrumb.
    await expect(page.getByText(/Stocktake|Stok opname/i).first()).toBeVisible();

    // The Add batches + Complete + Void buttons render in DRAFT mode.
    await expect(
      page.getByRole("button", { name: /Add batches|Tambah batch/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Complete|Selesaikan/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /^Void$|^Batalkan$/i }),
    ).toBeVisible();

    // Void the session so the next test run isn't blocked by the
    // single-DRAFT-globally rule.
    await page.getByRole("button", { name: /^Void$|^Batalkan$/i }).click();
    await expect(
      page.getByRole("button", { name: /Complete|Selesaikan/i }),
    ).toBeHidden();
  });
});
