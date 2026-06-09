import type { Page } from "@playwright/test";

import { clearAuth, expect, loginAs, OWNER, test } from "./_helpers";

// Coverage for spec bullet "Cashier Role — ensure cashier can open order
// history without error". A CASHIER must be able to land on /orders and
// /orders/:id without a 403 or console errors.

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

test.describe("cashier — order history access", () => {
  test("cashier opens /orders and /orders/:id without errors", async ({ page }) => {
    const m = String(Date.now());
    const cashierEmail = `cashier-${m}@apotech.local`;
    const cashierPassword = "Test1234!";

    let cashierId: string | undefined;
    let medId: string | undefined;
    try {
      // 1. As OWNER (default storageState): seed a cashier user + a completed
      //    sale so /orders has a real row to render.
      await page.goto("/");
      const u = await api<{ user: { id: string } }>(
        page,
        "user_iface.v1.UserService/CreateUser",
        {
          email: cashierEmail,
          name: `Cashier ${m}`,
          password: cashierPassword,
          role: 3 /* CASHIER */,
        },
      );
      cashierId = u.user.id;

      // Seed a medicine + batch + complete a sale so /orders isn't empty.
      const med = (await api<{ medicine: { id: string } }>(
        page,
        "inventory_iface.v1.MedicineService/CreateMedicine",
        { sku: `CO-${m}`, name: `Cashier Order Med ${m}`, unit: "tab", unitPrice: "1000" },
      )).medicine;
      medId = med.id;
      await api(page, "inventory_iface.v1.BatchService/CreateBatch", {
        medicineId: medId,
        batchNumber: `CO-B-${m}`,
        expiryDate: "2099-12-31",
        costPrice: "500",
        initialQuantity: "10",
      });
      const sale = (await api<{ sale: { id: string } }>(
        page,
        "pos_iface.v1.SaleService/StartSale",
        {},
      )).sale;
      await api(page, "pos_iface.v1.SaleService/AddItem", {
        saleId: sale.id,
        medicineId: medId,
        qty: 1,
      });
      await api(page, "pos_iface.v1.SaleService/CompleteSale", {
        saleId: sale.id,
        paymentSource: 1 /* CASH */,
        paidAmount: "1000",
      });

      // 2. Log out OWNER + log in as the seeded cashier.
      await clearAuth(page);
      await loginAs(page, {
        email: cashierEmail,
        password: cashierPassword,
        role: "CASHIER" as const,
      });

      // 3. Open /orders. Table renders + no console errors (the fixture
      //    guards on console errors).
      await page.goto("/orders");
      await page.waitForLoadState("networkidle");
      // The seeded medicine's row should be discoverable via search.
      await page
        .getByPlaceholder(/Search|Cari/i)
        .first()
        .fill(`Cashier Order Med ${m}`);
      await page.waitForTimeout(500);
      const row = page.getByRole("row").filter({ hasText: `Cashier Order Med ${m}` });
      await expect(row).toBeVisible();

      // 4. Click the row → /orders/:id renders.
      await row.click();
      await page.waitForURL(/\/orders\/[0-9a-f-]{36}$/);
      await expect(page.getByText(`Cashier Order Med ${m}`).first()).toBeVisible();
    } finally {
      // Cleanup: re-auth as OWNER and best-effort archive seeds.
      await clearAuth(page);
      await loginAs(page, OWNER);
      try {
        if (medId)
          await api(page, "inventory_iface.v1.MedicineService/ArchiveMedicine", { id: medId });
      } catch {
        /* */
      }
      try {
        if (cashierId)
          await api(page, "user_iface.v1.UserService/SetUserActive", {
            userId: cashierId,
            active: false,
          });
      } catch {
        /* */
      }
    }
  });
});
