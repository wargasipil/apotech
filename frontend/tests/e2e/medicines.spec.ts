import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// Coverage for spec bullets:
//   #3 ensure can create medicine (via the Add dialog, end-to-end)
//   #5 ensure can search sku, medicine name (debounced server search)
//   #6 ensure can filter last stock opname date (new "Opname before" filter)
//
// Seeding for #6 uses the JSON-over-Connect helpers (same pattern as
// pos-rx.spec.ts) so we can drive a COMPLETED stocktake without going through
// the stocktake UI.

async function api<T = unknown>(page: Page, path: string, body: unknown): Promise<T> {
  return await page.evaluate(
    async ([p, b]: [string, unknown]) => {
      const token = localStorage.getItem("apotech_access_token");
      if (!token) throw new Error("no access token");
      const res = await fetch(`/api/${p}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(b),
      });
      if (!res.ok) throw new Error(`${p}: ${res.status} ${await res.text()}`);
      return (await res.json()) as unknown;
    },
    [path, body] as const,
  ) as Promise<T>;
}

async function archiveMedicine(page: Page, id: string): Promise<void> {
  try {
    await api(page, "inventory_iface.v1.MedicineService/ArchiveMedicine", { id });
  } catch {
    /* best-effort */
  }
}

test.describe("medicines", () => {
  // Spec #3 — full UI flow: open the Add dialog, fill, save, find the new
  // row in the list.
  test("creates a medicine via the Add dialog and finds it in the list", async ({ page }) => {
    const m = String(Date.now());
    const sku = `MED-CR-${m}`;
    const name = `Created Med ${m}`;
    let createdId: string | undefined;

    try {
      await page.goto("/medicines");

      await page.getByRole("button", { name: /Add|Tambah/ }).first().click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();

      // Match by accessibility-name on role=textbox. The Chakra label "Base
      // unit *" contains a required-marker, and "Base unit" appears again
      // inside "Base price (per base unit)" — so `getByLabel` with a fuzzy
      // regex or `exact: true` is fragile. `getByRole("textbox", { name })`
      // uses the accessible name, which is the literal field name.
      await dialog.getByRole("textbox", { name: "SKU", exact: true }).fill(sku);
      await dialog.getByRole("textbox", { name: "Name", exact: true }).fill(name);
      await dialog.getByRole("textbox", { name: "Base unit", exact: true }).fill("tab");
      await dialog.getByRole("textbox", { name: /Base price/i }).fill("5000");

      await dialog.getByRole("button", { name: /Create|Save|Simpan/ }).click();
      await expect(dialog).toBeHidden();

      // The new SKU is unique → find via the list search; the table row links
      // out via the medicine id which we don't know yet. Use ListMedicines
      // to pull the id for cleanup.
      const listed = (await api<{ medicines: Array<{ id: string; sku: string }> }>(
        page,
        "inventory_iface.v1.MedicineService/ListMedicines",
        { query: sku, limit: 5 },
      )).medicines;
      expect(listed.length).toBeGreaterThan(0);
      const created = listed.find((mm) => mm.sku === sku);
      expect(created).toBeDefined();
      createdId = created!.id;

      // The new row is visible in the list table.
      await page.getByPlaceholder(/Search name or SKU|Cari/i).fill(sku);
      await expect(page.getByRole("cell", { name: sku })).toBeVisible();
    } finally {
      if (createdId) await archiveMedicine(page, createdId);
    }
  });

  // Spec #5 — debounced server-side search by SKU + name. Seeds two distinct
  // medicines so we can prove the search narrows specifically to one.
  test("server search filters by SKU and by name", async ({ page }) => {
    const m = String(Date.now());
    const seeds: Array<{ id: string; sku: string; name: string }> = [];
    try {
      // Navigate first so localStorage is accessible to the api helper
      // (about:blank denies storage access).
      await page.goto("/");
      for (let i = 0; i < 2; i++) {
        const sku = `MED-SR-${i}-${m}`;
        const name = `Search Med ${i} ${m}`;
        const r = (await api<{ medicine: { id: string } }>(
          page,
          "inventory_iface.v1.MedicineService/CreateMedicine",
          { sku, name, unit: "tab", unitPrice: "1000" },
        )).medicine;
        seeds.push({ id: r.id, sku, name });
      }

      await page.goto("/medicines");
      const search = page.getByPlaceholder(/Search name or SKU|Cari/i);

      // Search by SKU substring → only seed 0 matches.
      await search.fill(seeds[0].sku);
      await page.waitForTimeout(400); // debounce
      await expect(page.getByRole("cell", { name: seeds[0].sku })).toBeVisible();
      await expect(page.getByRole("cell", { name: seeds[1].sku })).toBeHidden();

      // Search by name substring → only seed 1 matches.
      await search.fill("");
      await search.fill(seeds[1].name);
      await page.waitForTimeout(400);
      await expect(page.getByRole("cell", { name: seeds[1].name })).toBeVisible();
      await expect(page.getByRole("cell", { name: seeds[0].name })).toBeHidden();
    } finally {
      for (const s of seeds) await archiveMedicine(page, s.id);
    }
  });

  // Spec #6 — new "Opname before" filter. Seed a medicine, complete a
  // stocktake on it today, then verify:
  //   - filter empty → medicine appears
  //   - filter = yesterday → medicine drops (last opname was today, not before yesterday)
  test("opname filter hides recently-counted medicines", async ({ page }) => {
    const m = String(Date.now());
    const sku = `MED-OP-${m}`;
    let medId: string | undefined;
    try {
      // Navigate first so localStorage is accessible to the api helper.
      await page.goto("/");
      const med = (await api<{ medicine: { id: string } }>(
        page,
        "inventory_iface.v1.MedicineService/CreateMedicine",
        { sku, name: `Opname Med ${m}`, unit: "tab", unitPrice: "1000" },
      )).medicine;
      medId = med.id;

      const batch = (await api<{ batch: { id: string } }>(
        page,
        "inventory_iface.v1.BatchService/CreateBatch",
        {
          medicineId: medId,
          batchNumber: `OP-B-${m}`,
          expiryDate: "2099-12-31",
          costPrice: "500",
          initialQuantity: "10",
        },
      )).batch;

      const sess = (await api<{ session: { id: string } }>(
        page,
        "stocktake_iface.v1.StocktakeService/StartStocktake",
        { name: `Opname filter ${m}` },
      )).session;
      await api(
        page,
        "stocktake_iface.v1.StocktakeService/AddBatchesToSession",
        { sessionId: sess.id, batchIds: [batch.id] },
      );
      const get = await api<{ lines: Array<{ id: string }> }>(
        page,
        "stocktake_iface.v1.StocktakeService/GetStocktake",
        { id: sess.id },
      );
      await api(
        page,
        "stocktake_iface.v1.StocktakeService/RecordCount",
        { lineId: get.lines[0].id, countedQty: 10 },
      );
      await api(
        page,
        "stocktake_iface.v1.StocktakeService/CompleteStocktake",
        { sessionId: sess.id },
      );

      await page.goto("/medicines");
      const search = page.getByPlaceholder(/Search name or SKU|Cari/i);
      await search.fill(sku);
      await page.waitForTimeout(400);
      // No filter yet → the medicine appears.
      await expect(page.getByRole("cell", { name: sku })).toBeVisible();

      // Set "Opname before" to yesterday → seeded medicine drops (its last
      // opname is today, which is NOT before yesterday).
      const yesterday = new Date(Date.now() - 24 * 3600 * 1000)
        .toISOString()
        .slice(0, 10);
      // DatePickerField renders an input that takes locale MM/DD/YYYY in en;
      // the Connect/proto value is YYYY-MM-DD. Find the date input by the
      // mm/dd/yyyy placeholder and fill the en form.
      const [yy, mo, da] = yesterday.split("-");
      const enDate = `${mo}/${da}/${yy}`;
      await page.getByPlaceholder("mm/dd/yyyy").fill(enDate);
      await page.getByPlaceholder("mm/dd/yyyy").blur();
      await page.waitForTimeout(400);
      await expect(page.getByRole("cell", { name: sku })).toBeHidden();

      // Clear the date → the medicine reappears.
      await page.getByPlaceholder("mm/dd/yyyy").fill("");
      await page.getByPlaceholder("mm/dd/yyyy").blur();
      await page.waitForTimeout(400);
      await expect(page.getByRole("cell", { name: sku })).toBeVisible();
    } finally {
      if (medId) await archiveMedicine(page, medId);
    }
  });

  // Spec: "ensure in medicine list have last opname date" — the new column on
  // /medicines. After a COMPLETED stocktake on a seeded medicine, today's
  // date should appear in that medicine's row.
  test("last opname date renders in the list after a completed stocktake", async ({ page }) => {
    const m = String(Date.now());
    const sku = `MED-LST-${m}`;
    let medId: string | undefined;
    try {
      await page.goto("/");
      const med = (await api<{ medicine: { id: string } }>(
        page,
        "inventory_iface.v1.MedicineService/CreateMedicine",
        { sku, name: `List Op Med ${m}`, unit: "tab", unitPrice: "1000" },
      )).medicine;
      medId = med.id;

      const batch = (await api<{ batch: { id: string } }>(
        page,
        "inventory_iface.v1.BatchService/CreateBatch",
        {
          medicineId: medId,
          batchNumber: `LST-B-${m}`,
          expiryDate: "2099-12-31",
          costPrice: "500",
          initialQuantity: "10",
        },
      )).batch;

      const sess = (await api<{ session: { id: string } }>(
        page,
        "stocktake_iface.v1.StocktakeService/StartStocktake",
        { name: `List op ${m}` },
      )).session;
      await api(
        page,
        "stocktake_iface.v1.StocktakeService/AddBatchesToSession",
        { sessionId: sess.id, batchIds: [batch.id] },
      );
      const get = await api<{ lines: Array<{ id: string }> }>(
        page,
        "stocktake_iface.v1.StocktakeService/GetStocktake",
        { id: sess.id },
      );
      await api(
        page,
        "stocktake_iface.v1.StocktakeService/RecordCount",
        { lineId: get.lines[0].id, countedQty: 10 },
      );
      await api(
        page,
        "stocktake_iface.v1.StocktakeService/CompleteStocktake",
        { sessionId: sess.id },
      );

      // Visit the list, narrow to our SKU, assert the seeded medicine's row
      // includes today's date in the new column.
      await page.goto("/medicines");
      await page.getByPlaceholder(/Search name or SKU|Cari/i).fill(sku);
      await page.waitForTimeout(400);
      // Local (not UTC) — the backend formats completed_at via time.Local;
      // toISOString lags by a day in UTC+ timezones late in the local day.
      const today = new Date().toLocaleDateString("sv");
      const row = page.getByRole("row").filter({ hasText: sku });
      await expect(row).toBeVisible();
      await expect(row.getByRole("cell", { name: today })).toBeVisible();
    } finally {
      if (medId) await archiveMedicine(page, medId);
    }
  });

  // Toolbar "Units" popover — sectioned by base unit. Selection is per-base
  // and stored in usePreferencesStore.medicineStockUnitsByBase. The medicine
  // seeded here has base "tab" with strip ×10 + box ×100 in its own units array,
  // so the popover renders a "tab" group even with an empty catalog (legacy
  // fallback via unitGroupsFromCatalog).
  test("stock unit popover renders ready stock in the chosen unit", async ({ page }) => {
    const m = String(Date.now());
    const baseName = `un-${m}`; // unique base name so the test is isolated
    const sku = `MED-UN-${m}`;
    let medId: string | undefined;
    try {
      await page.goto("/");

      const med = (await api<{ medicine: { id: string } }>(
        page,
        "inventory_iface.v1.MedicineService/CreateMedicine",
        {
          sku,
          name: `Unit Med ${m}`,
          unit: baseName,
          unitPrice: "1000",
          units: [
            { name: "strip", factor: "10", sellPrice: "9000", sellable: true, purchasable: true },
            { name: "box", factor: "100", sellPrice: "80000", sellable: true, purchasable: true },
          ],
        },
      )).medicine;
      medId = med.id;
      await api(
        page,
        "inventory_iface.v1.BatchService/CreateBatch",
        {
          medicineId: medId,
          batchNumber: `UN-B-${m}`,
          expiryDate: "2099-12-31",
          costPrice: "500",
          initialQuantity: "200",
        },
      );

      // Drive the per-base preference directly through the Zustand store. The
      // popover UI is asserted separately (opens, shows our group). Combining
      // popover clicks with section scoping was flaky against the per-base
      // layout — direct store writes prove the formatStock end-to-end path.
      const setPref = async (deriv: string) => {
        await page.evaluate(
          (args) => {
            const [bn, v] = args as [string, string];
            const k = "apotech_preferences";
            const raw = localStorage.getItem(k);
            const parsed = raw ? JSON.parse(raw) : { state: {}, version: 0 };
            parsed.state = parsed.state ?? {};
            parsed.state.medicineStockUnitsByBase = {
              ...(parsed.state.medicineStockUnitsByBase ?? {}),
              [bn]: v,
            };
            localStorage.setItem(k, JSON.stringify(parsed));
          },
          [baseName, deriv] as const,
        );
        await page.reload();
        await page.getByPlaceholder(/Search name or SKU|Cari/i).fill(sku);
        await page.waitForTimeout(400);
      };

      // Navigate to the list first so localStorage is on the right origin,
      // then drive the per-base preference + reload + re-fill.
      await page.goto("/medicines");
      await setPref("");
      const row = page.getByRole("row").filter({ hasText: sku });
      await expect(row).toBeVisible();
      const expectCell = (re: RegExp) =>
        expect(row.getByRole("cell", { name: re })).toBeVisible();
      await expectCell(new RegExp(`^200\\s+${baseName}$`));

      // box → "2 box"
      await setPref("box");
      await expectCell(/^2\s+box$/);

      // strip → "20 strip"
      await setPref("strip");
      await expectCell(/^20\s+strip$/);

      // Back to Base.
      await setPref("");
      await expectCell(new RegExp(`^200\\s+${baseName}$`));

      // Smoke test the popover button: clicking opens a dialog containing
      // our base's section heading.
      const unitsBtn = page.getByRole("button", { name: /Units|Satuan/i }).first();
      await unitsBtn.click();
      const popover = page.getByRole("dialog");
      await expect(popover).toBeVisible();
      await expect(popover.getByText(baseName, { exact: true })).toBeVisible();
      await page.keyboard.press("Escape");
    } finally {
      if (medId) await archiveMedicine(page, medId);
    }
  });
});
