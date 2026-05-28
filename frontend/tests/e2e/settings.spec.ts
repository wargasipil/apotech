import type { BrowserContext } from "@playwright/test";

import { expect, test } from "./_helpers";

// First Playwright coverage of the /settings page Backups section. The
// global.setup fixture pre-auths the OWNER, so /settings is reachable on goto.
//
// "Delete confirm dialog" skips on dev hosts where the backend can't reach
// pg_dump — the seed step API-calls CreateBackup the same way the UI does;
// if it fails, the test reports a clean skip rather than a misleading failure.
//
// Seeding happens on a SEPARATE page inside the same context: the global
// console-error fixture in _helpers.ts is bound to the test's `page`, so an
// expected 500 from CreateBackup (when pg_dump is missing) would otherwise
// fail the test as a "console error" instead of a clean skip.

type SeedResult = { ok: true; name: string } | { ok: false; reason: string };

async function seedBackup(context: BrowserContext): Promise<SeedResult> {
  const seedPage = await context.newPage();
  try {
    await seedPage.goto("/");
    return await seedPage.evaluate(async () => {
      const token = localStorage.getItem("apotech_access_token");
      if (!token) return { ok: false as const, reason: "no access token in localStorage" };
      const res = await fetch("/api/backup_iface.v1.BackupService/CreateBackup", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({}),
      });
      if (!res.ok) {
        const body = await res.text();
        return { ok: false as const, reason: `CreateBackup ${res.status}: ${body}` };
      }
      const data = (await res.json()) as { backup?: { name?: string } };
      if (!data.backup?.name) {
        return { ok: false as const, reason: "CreateBackup returned no name" };
      }
      return { ok: true as const, name: data.backup.name };
    });
  } finally {
    await seedPage.close();
  }
}

// Best-effort: delete a seeded backup via API on a throwaway page. Used in
// `finally` so a NotFound here (the UI already deleted) doesn't matter.
async function deleteBackupBestEffort(
  context: BrowserContext,
  name: string,
): Promise<void> {
  const p = await context.newPage();
  try {
    await p.goto("/");
    await p.evaluate(async (n) => {
      const token = localStorage.getItem("apotech_access_token");
      if (!token) return;
      await fetch("/api/backup_iface.v1.BackupService/DeleteBackup", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name: n }),
      }).catch(() => undefined);
    }, name);
  } finally {
    await p.close();
  }
}

test.describe("Settings — Backups section", () => {
  test("renders heading + Create button + help text", async ({ page }) => {
    await page.goto("/settings");
    // Section is below the low-stock-threshold form. The heading is the
    // primary regression guard: if the BackupsSection ever stops rendering,
    // this fails.
    await expect(
      page.getByRole("heading", { name: "Database backups" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Create backup" }),
    ).toBeVisible();
    // Help text proves the i18n key resolves (catches accidental key removal).
    await expect(
      page.getByText(/Snapshots are written to the server's backup directory/),
    ).toBeVisible();
  });

  test("Delete confirm dialog: cancel keeps the row, confirm removes it", async ({
    page,
    context,
  }) => {
    const seed = await seedBackup(context);
    test.skip(
      !seed.ok,
      `pg_dump unavailable on the backend host — skipping Delete-flow test (${seed.ok ? "" : seed.reason})`,
    );
    if (!seed.ok) return; // type narrow

    const backupName = seed.name;
    try {
      await page.goto("/settings");

      // The seeded backup appears as a row in the table.
      const row = page.getByRole("row", { name: new RegExp(backupName) });
      await expect(row).toBeVisible();

      // Open the confirm dialog from the row's Delete icon button.
      await row.getByRole("button", { name: "Delete" }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();
      await expect(
        dialog.getByRole("heading", { name: "Delete backup?" }),
      ).toBeVisible();
      // Body interpolates the backup name so the OWNER sees what's being deleted.
      await expect(dialog.getByText(backupName)).toBeVisible();

      // Cancel: dialog closes + row still present.
      await dialog.getByRole("button", { name: "Cancel" }).click();
      await expect(dialog).toBeHidden();
      await expect(row).toBeVisible();

      // Re-open + confirm: dialog closes + row gone.
      await row.getByRole("button", { name: "Delete" }).click();
      await expect(dialog).toBeVisible();
      await dialog.getByRole("button", { name: "Delete" }).click();
      await expect(dialog).toBeHidden();
      await expect(row).toBeHidden();
    } finally {
      await deleteBackupBestEffort(context, backupName);
    }
  });
});
