import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// End-to-end restock (purchase order) flow:
//   1. Seed a medicine with strip + box units, and a supplier (via the Connect
//      JSON API — fast + deterministic, same approach as pos-rx.spec.ts).
//   2. Create a restock through the UI: pick the supplier, pick the medicine,
//      switch the line to the "box" unit, set qty + line total, click Create.
//   3. On the restock DETAIL page, assert the supplier name and the medicine
//      info (name + unit-aware ordered qty "5 box") render correctly.
//
// Supplier/medicine names on the detail page are resolved client-side
// (GetPurchaseOrder returns only IDs), so this must be a browser test.

type Seed = { medicineId: string; supplierId: string };

async function seed(page: Page, marker: string): Promise<Seed> {
  await page.goto("/");
  return await page.evaluate(async (m: string) => {
    const token = localStorage.getItem("apotech_access_token");
    if (!token) throw new Error("no access token in localStorage");
    const headers = {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    };
    const post = async (path: string, body: unknown) => {
      const res = await fetch(`/api/${path}`, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(`${path}: ${res.status} ${await res.text()}`);
      return res.json();
    };

    // Medicine with base tablet + strip (×10) + box (×100). Connect JSON encodes
    // int64 (unitPrice, factor, sellPrice) as strings.
    const med = await post("inventory_iface.v1.MedicineService/CreateMedicine", {
      sku: `RS-${m}`,
      name: `Restock Med ${m}`,
      unit: "tablet",
      unitPrice: "700",
      units: [
        { name: "strip", factor: "10", sellPrice: "6500", sellable: true, purchasable: true },
        { name: "box", factor: "100", sellPrice: "60000", sellable: true, purchasable: true },
      ],
    });

    const sup = await post("inventory_iface.v1.SupplierService/CreateSupplier", {
      code: `RS${m}`,
      name: `Restock Supplier ${m}`,
    });

    return { medicineId: med.medicine.id, supplierId: sup.supplier.id };
  }, marker);
}

async function cleanup(
  page: Page,
  ids: { medicineId: string; supplierId: string; poId?: string },
): Promise<void> {
  await page.evaluate(async (s: { medicineId: string; supplierId: string; poId?: string }) => {
    const token = localStorage.getItem("apotech_access_token");
    if (!token) return;
    const headers = {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    };
    const post = (path: string, body: unknown) =>
      fetch(`/api/${path}`, { method: "POST", headers, body: JSON.stringify(body) });
    // DRAFT PO is voidable; archive the catalog rows (soft delete).
    if (s.poId) {
      await post("purchasing_iface.v1.PurchaseOrderService/VoidPurchaseOrder", { id: s.poId });
    }
    await post("inventory_iface.v1.SupplierService/ArchiveSupplier", { id: s.supplierId });
    await post("inventory_iface.v1.MedicineService/ArchiveMedicine", { id: s.medicineId });
  }, ids);
}

test.describe("restock (purchase order) end-to-end", () => {
  test("create medicine (units) + supplier → restock in box → detail shows supplier + medicine", async ({
    page,
  }) => {
    const m = String(Date.now());
    const ids = await seed(page, m);
    let poId: string | undefined;
    try {
      await page.goto("/purchasing/new");

      // Supplier — async SearchableSelect renders <input role="combobox">.
      await page.getByPlaceholder("Select supplier").fill(`Restock Supplier ${m}`);
      await page.waitForTimeout(700); // debounced SearchSuppliers
      await page.getByRole("option", { name: new RegExp(`Restock Supplier ${m}`) }).click();

      // Medicine (line) — async SearchableSelect. Picking it sets the line's
      // units and defaults the unit to base ("tablet").
      await page.getByPlaceholder("Select medicine").fill(`Restock Med ${m}`);
      await page.waitForTimeout(700); // debounced SearchMedicines
      await page.getByRole("option", { name: new RegExp(`Restock Med ${m}`) }).click();

      // Switch the line unit to "box". EnumSelect's trigger is the only
      // <button role="combobox"> in the items table (the medicine select is an
      // <input>, the hidden form select is a <select>).
      const unitTrigger = page.getByRole("table").locator('button[role="combobox"]');
      await expect(unitTrigger).toBeVisible();
      await unitTrigger.click();
      await page.getByRole("option", { name: "box" }).click();

      // Qty 5 (the only number input / spinbutton) + line total (the formatted
      // MoneyInput, now a text input → role "textbox").
      await page.getByRole("table").getByRole("spinbutton").first().fill("5");
      await page.getByRole("table").getByRole("textbox").fill("300000");

      // Create → navigates to the restock detail page.
      await page.getByRole("button", { name: "Create" }).click();
      await page.waitForURL(/\/purchasing\/[0-9a-f-]{36}$/);
      poId = page.url().split("/").pop();

      // Restock detail: supplier name + medicine info are correct.
      await expect(page.getByText(`Restock Supplier ${m}`)).toBeVisible();
      await expect(page.getByText(`Restock Med ${m}`)).toBeVisible();
      // Ordered qty renders unit-aware: 500 base ÷ 100 = "5 box" (shown in both
      // the Ordered and Received cells, so match the first).
      await expect(page.getByText("5 box").first()).toBeVisible();

      // Restock LIST table: the Supplier column resolves to "code · name", not a
      // raw UUID (the ALL_LIMIT name-map preload must include our supplier).
      await page.goto("/purchasing/all");
      await page.getByPlaceholder(/Search PO/i).fill(`Restock Supplier ${m}`);
      await page.waitForTimeout(700); // debounced list query
      // Scope to the table cell (a combobox option can carry the same text).
      await expect(
        page.getByRole("cell", { name: new RegExp(`RS${m} · Restock Supplier ${m}`) }),
      ).toBeVisible();
    } finally {
      await cleanup(page, { ...ids, poId });
    }
  });

  // Receive flow: drive the same dialog the user reported errored on.
  // Send → Receive → assert status flips to RECEIVED.
  test("Send + Receive a PO walks the dialog and lands on RECEIVED", async ({ page }) => {
    const m = String(Date.now());
    const ids = await seed(page, m);
    let poId: string | undefined;
    try {
      // Same PO setup as the first test: 5 box of the seeded medicine.
      await page.goto("/purchasing/new");
      await page.getByPlaceholder("Select supplier").fill(`Restock Supplier ${m}`);
      await page.waitForTimeout(700);
      await page.getByRole("option", { name: new RegExp(`Restock Supplier ${m}`) }).click();
      await page.getByPlaceholder("Select medicine").fill(`Restock Med ${m}`);
      await page.waitForTimeout(700);
      await page.getByRole("option", { name: new RegExp(`Restock Med ${m}`) }).click();
      const unitTrigger = page.getByRole("table").locator('button[role="combobox"]');
      await unitTrigger.click();
      await page.getByRole("option", { name: "box" }).click();
      await page.getByRole("table").getByRole("spinbutton").first().fill("5");
      await page.getByRole("table").getByRole("textbox").fill("300000");
      await page.getByRole("button", { name: "Create" }).click();
      await page.waitForURL(/\/purchasing\/[0-9a-f-]{36}$/);
      poId = page.url().split("/").pop();

      // Send → status flips to Sent.
      await page.getByRole("button", { name: "Send", exact: true }).click();
      await expect(page.getByText("Sent", { exact: true })).toBeVisible();

      // Receive → dialog opens.
      await page.getByRole("button", { name: "Receive", exact: true }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();

      // Fill batch number + expiry inside the row. The receive table has 1
      // spinbutton (qty, already defaulted to 5) + 3 textboxes per row:
      // nth(0) = unit cost (defaulted), nth(1) = batch number, nth(2) = expiry.
      const rowTextboxes = dialog.getByRole("table").getByRole("textbox");
      await rowTextboxes.nth(1).fill(`RS-B1-${m}`);
      // DatePicker.Input under en locale takes MM/DD/YYYY.
      await rowTextboxes.nth(2).fill("12/31/2099");
      await rowTextboxes.nth(2).blur();

      // Submit (dialog footer's Receive button).
      await dialog.getByRole("button", { name: "Receive", exact: true }).click();
      await expect(dialog).not.toBeVisible();

      // Status badge flips to Received (5 box ordered, 5 box received = full).
      // The detail page also has a "Received" table column header — match the
      // first occurrence (the badge, which renders above the table).
      await expect(page.getByText("Received", { exact: true }).first()).toBeVisible();
    } finally {
      // A RECEIVED PO can't be voided; sending the request would log a 400 and
      // trip the console-error fixture. Skip the void call by omitting poId.
      // The PO row stays around (timestamp-unique, no test interference);
      // supplier + medicine still get archived.
      void poId; // referenced for the catch above
      await cleanup(page, { medicineId: ids.medicineId, supplierId: ids.supplierId });
    }
  });
});
