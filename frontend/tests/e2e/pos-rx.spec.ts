import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// Regression spec for the POS Rx-required auto-pick flow:
// clicking an Rx-required medicine without a prescription attached should
// open the PrescriptionPickerDialog (filtered to that medicine), and picking
// an Rx should atomically attach it + add the medicine to the cart.
//
// Seeds the needed rows via Connect's JSON protocol, hitting the backend
// over the same /api proxy the app uses. The bootstrap-owner token is read
// from localStorage (storageState fixture).

type SeedIds = { medicineId: string; customerId: string; prescriptionId: string };

async function seedRxScenario(page: Page, marker: string): Promise<SeedIds> {
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

    const med = await post(
      "inventory_iface.v1.MedicineService/CreateMedicine",
      {
        sku: `RX-${m}`,
        name: `Rx Med ${m}`,
        unit: "tab",
        unitPrice: "5000",
        prescriptionRequired: true,
      },
    );
    // Give the Rx medicine some stock so the search row is clickable.
    await post("inventory_iface.v1.BatchService/CreateBatch", {
      medicineId: med.medicine.id,
      batchNumber: `BATCH-${m}`,
      expiryDate: "2099-12-31",
      costPrice: "2000",
      initialQuantity: "20",
    });

    const cust = await post(
      "customer_iface.v1.CustomerService/CreateCustomer",
      { name: `Rx Patient ${m}` },
    );
    const today = new Date().toISOString().slice(0, 10);
    const rx = await post(
      "prescription_iface.v1.PrescriptionService/CreatePrescription",
      {
        customerId: cust.customer.id,
        issuerName: `Dr. ${m}`,
        issuedAt: today,
        items: [{ medicineId: med.medicine.id, prescribedQty: 10 }],
      },
    );

    return {
      medicineId: med.medicine.id,
      customerId: cust.customer.id,
      prescriptionId: rx.prescription.id,
    };
  }, marker);
}

async function archiveSeed(page: Page, ids: SeedIds): Promise<void> {
  await page.evaluate(async (s: SeedIds) => {
    const token = localStorage.getItem("apotech_access_token");
    if (!token) return;
    const headers = {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    };
    const post = (path: string, body: unknown) =>
      fetch(`/api/${path}`, { method: "POST", headers, body: JSON.stringify(body) });
    await post("prescription_iface.v1.PrescriptionService/VoidPrescription", {
      id: s.prescriptionId,
    });
    await post("customer_iface.v1.CustomerService/ArchiveCustomer", { id: s.customerId });
    await post("inventory_iface.v1.MedicineService/ArchiveMedicine", { id: s.medicineId });
  }, ids);
}

test.describe("POS Rx-required auto-pick", () => {
  test("clicking Rx-required medicine opens picker, picking adds to cart", async ({ page }) => {
    const marker = String(Date.now());
    const ids = await seedRxScenario(page, marker);
    try {
      await page.goto("/pos");
      // Wait for StartSale to land so we have a draft sale.
      await page.waitForLoadState("networkidle");

      // Search the seeded medicine. Use the SKU so we match exactly.
      const searchInput = page.getByPlaceholder(/search medicine|cari obat/i);
      await searchInput.fill(`RX-${marker}`);

      // Click the result row.
      await page.getByText(`Rx Med ${marker}`).first().click();

      // Picker should open with the "Need prescription for" caption.
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();
      await expect(dialog.getByText(/Need prescription for|Perlu resep untuk/)).toBeVisible();
      // Our seeded Rx covers this medicine and shows up in the (now filtered) list.
      await expect(dialog.getByText(`Dr. ${marker}`)).toBeVisible();

      // Pick the Rx — should auto-attach + auto-add the medicine.
      await dialog.getByText(`Dr. ${marker}`).click();

      // Dialog dismisses and the cart shows the medicine. Two elements
      // with this name will be on the page now (the search row stays + the
      // cart row appears), so assert on the count rather than .toBeVisible.
      await expect(dialog).toBeHidden();
      await expect(page.getByText(`Rx Med ${marker}`)).toHaveCount(2);
    } finally {
      await archiveSeed(page, ids);
    }
  });

  test("picker shows the seeded prescription with per-item remaining", async ({ page }) => {
    const marker = String(Date.now() + 1);
    const ids = await seedRxScenario(page, marker);
    try {
      await page.goto("/pos");
      await page.waitForLoadState("networkidle");

      // Open the picker manually (no pending-add): F5.
      await page.keyboard.press("F5");
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();

      // Per-item row renders "name  10/10 remaining".
      await expect(dialog.getByText(/10\/10\s+remaining|10\/10\s+sisa/)).toBeVisible();
    } finally {
      await archiveSeed(page, ids);
    }
  });
});
