import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// Coverage for spec bullets:
//   #8 POS — ensure can checkout (search → cart → complete → receipt)
//   #9 POS — ensure stock decrease properly + pricing snapshots properly
//
// The race-condition guarantee (#10) is covered at the Go layer
// (`TestConcurrentCompleteSale_NoOversell`); reproducing concurrent
// CompleteSale in Playwright is flaky.

type Seed = { medicineId: string; sku: string; name: string; batchId: string };

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

async function seed(page: Page, marker: string, initialQty: number): Promise<Seed> {
  await page.goto("/");
  const sku = `POS-CK-${marker}`;
  const name = `POS Checkout Med ${marker}`;
  const med = (await api<{ medicine: { id: string } }>(
    page,
    "inventory_iface.v1.MedicineService/CreateMedicine",
    { sku, name, unit: "tab", unitPrice: "5000" },
  )).medicine;
  const batch = (await api<{ batch: { id: string } }>(
    page,
    "inventory_iface.v1.BatchService/CreateBatch",
    {
      medicineId: med.id,
      batchNumber: `POS-CK-B-${marker}`,
      expiryDate: "2099-12-31",
      costPrice: "2000",
      initialQuantity: String(initialQty),
    },
  )).batch;
  return { medicineId: med.id, sku, name, batchId: batch.id };
}

async function archive(page: Page, s: Seed): Promise<void> {
  try {
    await api(page, "inventory_iface.v1.MedicineService/ArchiveMedicine", { id: s.medicineId });
  } catch {
    /* */
  }
}

test.describe("POS checkout", () => {
  test("full sale: search → add → F8 complete → receipt + stock decreases + price snapshots", async ({
    page,
  }) => {
    const marker = String(Date.now());
    const initialQty = 10;
    const s = await seed(page, marker, initialQty);
    try {
      await page.goto("/pos");
      await page.waitForLoadState("networkidle");

      // Search by SKU and click the medicine row.
      const searchBox = page.getByPlaceholder(/search medicine|cari obat/i);
      await searchBox.fill(s.sku);
      await page.waitForTimeout(400); // POS uses client-side filter; brief tick is plenty
      await page.getByText(s.name).first().click();

      // Cart row appears with qty 1 (the on-add default).
      await expect(page.getByText(s.name)).toHaveCount(2); // search row + cart row

      // CASH payment requires `paidAmount >= total`; default is 0. Set it
      // to the total so F8 actually fires CompleteSale.
      const paidInput = page.getByText(/^Paid$|^Bayar$/).locator("..").getByRole("textbox");
      await paidInput.fill("5000");

      // F8 → CompleteSale → receipt dialog appears.
      await page.keyboard.press("F8");
      const receipt = page.getByRole("dialog");
      await expect(receipt).toBeVisible({ timeout: 5000 });
      // Receipt title contains "Receipt" / "Struk".
      await expect(receipt.getByText(/Receipt|Struk/i)).toBeVisible();
      // Line for our medicine, with the 5000 unit price snapshot.
      await expect(receipt.getByText(s.name)).toBeVisible();

      // Verify the stock decreased by 1.
      const after = (await api<{ medicine: { readyStock: string } }>(
        page,
        "inventory_iface.v1.MedicineService/GetMedicine",
        { id: s.medicineId },
      )).medicine;
      expect(Number(after.readyStock)).toBe(initialQty - 1);

      // Pricing snapshot: the most recent sale of this medicine carries the
      // unit_price_snapshot we set at CreateMedicine time (5000). ListSales
      // takes ILIKE on sale_no / customer name / medicine name — query by
      // medicine name. Connect-JSON may omit empty repeated fields, so guard
      // with ?? [].
      type SaleItem = {
        medicineId: string;
        unitPriceSnapshot?: string;
        qty?: number;
        lineTotal?: string;
      };
      const sales = await api<{ sales?: Array<{ id: string; items?: SaleItem[] }> }>(
        page,
        "pos_iface.v1.SaleService/ListSales",
        { query: s.name, limit: 5 },
      );
      const ourSale = (sales.sales ?? []).find((sale) =>
        (sale.items ?? []).some((it) => it.medicineId === s.medicineId),
      );
      expect(ourSale).toBeDefined();
      const ourLine = (ourSale!.items ?? []).find((it) => it.medicineId === s.medicineId);
      expect(ourLine).toBeDefined();
      expect(Number(ourLine!.unitPriceSnapshot ?? 0)).toBe(5000);
      expect(Number(ourLine!.qty ?? 0)).toBe(1);
      expect(Number(ourLine!.lineTotal ?? 0)).toBe(5000);

      // Spec: "ensure after checkout is exist in order history, and check data
      // is correct". Navigate to /orders and verify the sale appears with
      // correct total + medicine. Click into detail and assert the line.
      await page.goto("/orders");
      await page.getByPlaceholder(/Search|Cari/i).first().fill(s.name);
      await page.waitForTimeout(500); // debounced ListSales
      // The order row carries the medicine name (denormalized) and total
      // 5,000. Scope to the row, then click to drill into the detail page.
      const row = page.getByRole("row").filter({ hasText: s.name });
      await expect(row).toBeVisible();
      await expect(row.getByText(/5[.,]000/)).toBeVisible();
      await row.click();
      await page.waitForURL(/\/orders\/[0-9a-f-]{36}$/);
      // Detail page renders the medicine in the items table + the total card.
      await expect(page.getByText(s.name).first()).toBeVisible();
      await expect(page.getByText(/5[.,]000/).first()).toBeVisible();

      // Spec (Order history): cancel sold + stock restored. Click "Cancel
      // order" → ConfirmDialog appears → confirm → status flips to Voided +
      // ready stock back to initial.
      await page.getByRole("button", { name: /Cancel order|Batalkan order/i }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();
      await dialog
        .getByRole("button", { name: /Cancel order|Batalkan order/i })
        .click();
      await expect(dialog).toBeHidden();
      // Status badge flips to Voided.
      await expect(page.getByText(/^Voided$|^Batal$/).first()).toBeVisible();

      // Stock back to initial via the API.
      const restored = (await api<{ medicine: { readyStock: string } }>(
        page,
        "inventory_iface.v1.MedicineService/GetMedicine",
        { id: s.medicineId },
      )).medicine;
      expect(Number(restored.readyStock)).toBe(initialQty);
    } finally {
      await archive(page, s);
    }
  });
});
