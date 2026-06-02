import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// Coverage for spec bullets:
//   batch: ensure can filter by supplier (NEW feature in this commit)
//   batch: ensure have link to detail restock (NEW: batch → PO link)
//
// Seeds via the JSON-over-Connect helper (same pattern as restock.spec.ts).

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

async function archive(page: Page, ids: { medicineId: string; supplierIds: string[] }): Promise<void> {
  try {
    await api(page, "inventory_iface.v1.MedicineService/ArchiveMedicine", { id: ids.medicineId });
  } catch {
    /* */
  }
  for (const sid of ids.supplierIds) {
    try {
      await api(page, "inventory_iface.v1.SupplierService/ArchiveSupplier", { id: sid });
    } catch {
      /* */
    }
  }
}

test.describe("batches", () => {
  // Spec: filter by supplier — newly added <SearchableSelect> in the toolbar.
  test("supplier filter narrows the batches list", async ({ page }) => {
    const m = String(Date.now());
    let med: { id: string } | undefined;
    const supplierIds: string[] = [];
    try {
      await page.goto("/");

      const supA = (await api<{ supplier: { id: string; code: string; name: string } }>(
        page,
        "inventory_iface.v1.SupplierService/CreateSupplier",
        { code: `BFA${m}`.slice(0, 12), name: `Batch Sup A ${m}` },
      )).supplier;
      supplierIds.push(supA.id);
      const supB = (await api<{ supplier: { id: string; code: string; name: string } }>(
        page,
        "inventory_iface.v1.SupplierService/CreateSupplier",
        { code: `BFB${m}`.slice(0, 12), name: `Batch Sup B ${m}` },
      )).supplier;
      supplierIds.push(supB.id);

      med = (await api<{ medicine: { id: string } }>(
        page,
        "inventory_iface.v1.MedicineService/CreateMedicine",
        { sku: `BF-${m}`, name: `Batch Filter Med ${m}`, unit: "tab", unitPrice: "1000" },
      )).medicine;

      const batchNoA = `BFA-${m}`;
      const batchNoB = `BFB-${m}`;
      await api(page, "inventory_iface.v1.BatchService/CreateBatch", {
        medicineId: med.id,
        supplierId: supA.id,
        batchNumber: batchNoA,
        expiryDate: "2099-12-31",
        costPrice: "100",
        initialQuantity: "5",
      });
      await api(page, "inventory_iface.v1.BatchService/CreateBatch", {
        medicineId: med.id,
        supplierId: supB.id,
        batchNumber: batchNoB,
        expiryDate: "2099-12-31",
        costPrice: "100",
        initialQuantity: "5",
      });

      await page.goto("/inventory/batches");
      // Search by medicine name to keep noise low; both batches now visible.
      await page.getByPlaceholder(/Search medicine|Cari/i).fill(`Batch Filter Med ${m}`);
      await page.waitForTimeout(400);
      await expect(page.getByRole("cell", { name: batchNoA })).toBeVisible();
      await expect(page.getByRole("cell", { name: batchNoB })).toBeVisible();

      // Open the supplier filter (the only Supplier-placeholder SearchableSelect),
      // type supA's code, click the option.
      const supplierFilter = page.getByPlaceholder(/^Supplier$|^Pemasok$/);
      await supplierFilter.fill(supA.code);
      await page.waitForTimeout(400);
      await page.getByRole("option", { name: new RegExp(`${supA.code}`) }).click();
      await page.waitForTimeout(300); // debounce
      await expect(page.getByRole("cell", { name: batchNoA })).toBeVisible();
      await expect(page.getByRole("cell", { name: batchNoB })).toBeHidden();
    } finally {
      if (med) await archive(page, { medicineId: med.id, supplierIds });
    }
  });

  // Spec: link to detail restock — the new "PO" column links each batch row
  // back to its originating purchase-order detail page.
  test("batch row links to its PO detail", async ({ page }) => {
    const m = String(Date.now());
    let med: { id: string } | undefined;
    const supplierIds: string[] = [];
    try {
      await page.goto("/");

      const sup = (await api<{ supplier: { id: string; code: string; name: string } }>(
        page,
        "inventory_iface.v1.SupplierService/CreateSupplier",
        { code: `BPK${m}`.slice(0, 12), name: `Batch PO link Sup ${m}` },
      )).supplier;
      supplierIds.push(sup.id);

      med = (await api<{ medicine: { id: string } }>(
        page,
        "inventory_iface.v1.MedicineService/CreateMedicine",
        { sku: `BPK-${m}`, name: `Batch PO link Med ${m}`, unit: "tab", unitPrice: "1000" },
      )).medicine;

      // Create + send + receive a PO. The receipt creates the batch with PO linkage.
      const po = (await api<{ order: { id: string; poNo: string; items: Array<{ id: string }> } }>(
        page,
        "purchasing_iface.v1.PurchaseOrderService/CreatePurchaseOrder",
        {
          supplierId: sup.id,
          items: [{ medicineId: med.id, orderedQty: 5, unitCostPrice: "1000" }],
        },
      )).order;
      await api(page, "purchasing_iface.v1.PurchaseOrderService/SendPurchaseOrder", { id: po.id });
      const batchNo = `BPK-B-${m}`;
      await api(page, "purchasing_iface.v1.PurchaseReceiptService/CreateReceipt", {
        purchaseOrderId: po.id,
        lines: [
          { purchaseOrderItemId: po.items[0].id, qty: 5, expiryDate: "2099-12-31", batchNumber: batchNo },
        ],
      });

      await page.goto("/inventory/batches");
      await page.getByPlaceholder(/Search medicine|Cari/i).fill(`Batch PO link Med ${m}`);
      await page.waitForTimeout(400);
      const row = page.getByRole("row").filter({ hasText: batchNo });
      await expect(row).toBeVisible();
      // The PO link cell carries po.poNo as the link text.
      await row.getByRole("link", { name: po.poNo }).click();
      await page.waitForURL(new RegExp(`/purchasing/${po.id}$`));
      // Detail page heading shows the PO no.
      await expect(page.getByRole("heading", { name: po.poNo })).toBeVisible();
    } finally {
      if (med) await archive(page, { medicineId: med.id, supplierIds });
    }
  });
});
