-- +goose Up
-- Backfills are no-ops on a fresh portable DB; service layer populates these
-- columns on every CreatePurchaseOrder + CreateReceipt.
ALTER TABLE purchase_order_items ADD COLUMN medicine_unit_id TEXT;
ALTER TABLE purchase_order_items ADD COLUMN unit_name TEXT NOT NULL DEFAULT '';
ALTER TABLE purchase_order_items ADD COLUMN unit_factor INTEGER NOT NULL DEFAULT 1;

ALTER TABLE purchase_receipt_items ADD COLUMN medicine_unit_id TEXT;
ALTER TABLE purchase_receipt_items ADD COLUMN unit_name TEXT NOT NULL DEFAULT '';
ALTER TABLE purchase_receipt_items ADD COLUMN unit_factor INTEGER NOT NULL DEFAULT 1;

-- +goose Down
SELECT 1;
