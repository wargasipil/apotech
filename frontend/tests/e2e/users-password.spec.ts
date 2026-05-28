import { expect, test } from "./_helpers";

// Change password: shared <ChangePasswordDialog> opened from the TopBar user
// menu (self path). Backend ChangePassword (users.go:201-240) already covers
// both self + OWNER-changes-other branches; this spec verifies the UI wiring.
//
// We don't actually Save here: the seeded bootstrap password "test123" is 7
// chars, below the new-password minimum (8), so a round-trip rotate → restore
// can't preserve the seed. Verifying the dialog open/fields/validation/close
// pins the UI without leaving the user in a rotated state.

test.describe("change password", () => {
  test("TopBar menu opens the password dialog with current/new/confirm fields", async ({
    page,
  }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "user menu" }).click();
    await page.getByRole("menu").getByText(/Change my password/i).click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();

    // Three password inputs: current, new, confirm.
    const inputs = dialog.locator('input[type="password"]');
    await expect(inputs).toHaveCount(3);

    // Save is disabled with empty fields.
    const save = dialog.getByRole("button", { name: "Save" });
    await expect(save).toBeDisabled();

    // Fill mismatched new/confirm → validation message + still disabled.
    await inputs.nth(0).fill("test123");
    await inputs.nth(1).fill("abcd1234");
    await inputs.nth(2).fill("zzzz9999");
    await expect(dialog.getByText(/Passwords do not match|tidak cocok/i)).toBeVisible();
    await expect(save).toBeDisabled();

    // Fix the confirm → validation clears + Save enables.
    await inputs.nth(2).fill("abcd1234");
    await expect(dialog.getByText(/Passwords do not match|tidak cocok/i)).toHaveCount(0);
    await expect(save).toBeEnabled();

    // Cancel — no rotation happens, dialog closes.
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).not.toBeVisible();
  });
});
