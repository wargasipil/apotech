import { expect, test } from "./_helpers";

test.describe("EntityDrawer (slide-over)", () => {
  test("Customers Add → Cancel closes the drawer", async ({ page }) => {
    await page.goto("/customers");
    await page.getByRole("button", { name: "Add" }).click();
    const drawer = page.getByRole("dialog");
    await expect(drawer).toBeVisible();
    await expect(drawer.getByRole("heading", { name: "Add customer" })).toBeVisible();
    await drawer.getByRole("button", { name: "Cancel" }).click();
    await expect(drawer).toBeHidden();
  });

  test("Customers Add → Save creates a row and closes", async ({ page }) => {
    // Unique name per run so suites stay independent.
    const name = `e2e-customer-${Date.now()}`;
    await page.goto("/customers");
    await page.getByRole("button", { name: "Add" }).click();
    const drawer = page.getByRole("dialog");
    await drawer.getByRole("textbox", { name: "Name" }).fill(name);
    await drawer.getByRole("button", { name: "Save" }).click();
    await expect(drawer).toBeHidden();
    await expect(page.getByRole("cell", { name })).toBeVisible();

    // Cleanup: archive the row so future runs stay clean.
    const row = page.getByRole("row", { name: new RegExp(name) });
    await row.getByRole("button", { name: "Archive" }).click();
  });

  test("Branches Add → Save is disabled until required fields are filled", async ({ page }) => {
    await page.goto("/branches");
    await page.getByRole("button", { name: "Add" }).click();
    const drawer = page.getByRole("dialog");
    const save = drawer.getByRole("button", { name: "Save" });
    await expect(save).toBeDisabled();

    const inputs = drawer.locator("input");
    await inputs.nth(0).fill("E2E01"); // code
    await expect(save).toBeDisabled(); // name still empty
    await inputs.nth(1).fill("E2E branch"); // name
    await expect(save).toBeEnabled();

    await drawer.getByRole("button", { name: "Cancel" }).click();
    await expect(drawer).toBeHidden();
  });

  test.fixme(
    "EntityDrawer resets form state when re-opened after Cancel",
    async ({ page }) => {
      // Known bug: closing without saving leaves the previously-typed values
      // sitting in the form on next open. The fix is to reset RHF on close
      // (or remount via key={open}) across every drawer. Flip .fixme to ()
      // when fixed.
      await page.goto("/customers");
      await page.getByRole("button", { name: "Add" }).click();
      await page
        .getByRole("dialog")
        .getByRole("textbox", { name: "Name" })
        .fill("Ghost dummy");
      await page.getByRole("dialog").getByRole("button", { name: "Cancel" }).click();
      await page.getByRole("button", { name: "Add" }).click();
      const value = await page
        .getByRole("dialog")
        .getByRole("textbox", { name: "Name" })
        .inputValue();
      expect(value).toBe("");
    },
  );
});

test.describe("Dialog (centered modal)", () => {
  test("F4 on POS opens the customer picker and auto-focuses the search", async ({ page }) => {
    await page.goto("/pos");
    // StartSale fires on mount; give it a beat so the page is interactive.
    await page.waitForLoadState("networkidle");
    await page.keyboard.press("F4");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "Attach customer" })).toBeVisible();
    // Search input is auto-focused.
    const focusedPlaceholder = await page.evaluate(
      () => (document.activeElement as HTMLInputElement | null)?.placeholder ?? "",
    );
    expect(focusedPlaceholder).toMatch(/search/i);
  });

  test("Dialog dismisses via Escape, close button, and outside-positioner click", async ({
    page,
  }) => {
    await page.goto("/pos");
    await page.waitForLoadState("networkidle");

    // 1. Escape closes
    await page.keyboard.press("F4");
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toBeHidden();

    // 2. Close [×] button closes
    await page.keyboard.press("F4");
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.getByRole("dialog").getByRole("button", { name: "close" }).click();
    await expect(page.getByRole("dialog")).toBeHidden();

    // 3. Outside-positioner click closes. The visible "backdrop" is shielded
    // by the positioner div which is what Chakra listens on for outside
    // clicks; we synthesize the pointer events at coordinates outside the
    // content card.
    await page.keyboard.press("F4");
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.evaluate(() => {
      const target = document.elementFromPoint(50, 500) as HTMLElement | null;
      const opts = { bubbles: true, clientX: 50, clientY: 500, button: 0 };
      target?.dispatchEvent(new PointerEvent("pointerdown", opts));
      target?.dispatchEvent(new MouseEvent("mousedown", opts));
      target?.dispatchEvent(new MouseEvent("mouseup", opts));
      target?.dispatchEvent(new PointerEvent("pointerup", opts));
      target?.dispatchEvent(new MouseEvent("click", opts));
    });
    await expect(page.getByRole("dialog")).toBeHidden();
  });
});
