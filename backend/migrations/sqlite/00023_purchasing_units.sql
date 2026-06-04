-- +goose Up
-- Phase 2 buy-in-units: PO + receipt lines remember the purchasable unit they
-- were entered in. Quantities (ordered_qty / received_qty / qty) stay in BASE
-- units; these columns are display/entry metadata (unit_factor = base per 1).
-- One ADD COLUMN per statement (SQLite has no multi-column ALTER).
ALTER TABLE purchase_order_items ADD COLUMN medicine_unit_id TEXT;
ALTER TABLE purchase_order_items ADD COLUMN unit_name TEXT NOT NULL DEFAULT '';
ALTER TABLE purchase_order_items ADD COLUMN unit_factor INTEGER NOT NULL DEFAULT 1;

ALTER TABLE purchase_receipt_items ADD COLUMN medicine_unit_id TEXT;
ALTER TABLE purchase_receipt_items ADD COLUMN unit_name TEXT NOT NULL DEFAULT '';
ALTER TABLE purchase_receipt_items ADD COLUMN unit_factor INTEGER NOT NULL DEFAULT 1;

-- Backfill existing rows to each medicine's base unit (no-op on a fresh DB).
-- Correlated subqueries replace Postgres UPDATE ... FROM.
UPDATE purchase_order_items SET
  medicine_unit_id = (SELECT mu.id FROM medicine_units mu WHERE mu.medicine_id = purchase_order_items.medicine_id AND mu.is_base),
  unit_name        = (SELECT mu.name FROM medicine_units mu WHERE mu.medicine_id = purchase_order_items.medicine_id AND mu.is_base)
WHERE medicine_unit_id IS NULL;

UPDATE purchase_receipt_items SET
  medicine_unit_id = (SELECT mu.id FROM medicine_units mu WHERE mu.medicine_id = purchase_receipt_items.medicine_id AND mu.is_base),
  unit_name        = (SELECT mu.name FROM medicine_units mu WHERE mu.medicine_id = purchase_receipt_items.medicine_id AND mu.is_base)
WHERE medicine_unit_id IS NULL;

-- +goose Down
ALTER TABLE purchase_order_items DROP COLUMN unit_factor;
ALTER TABLE purchase_order_items DROP COLUMN unit_name;
ALTER TABLE purchase_order_items DROP COLUMN medicine_unit_id;
ALTER TABLE purchase_receipt_items DROP COLUMN unit_factor;
ALTER TABLE purchase_receipt_items DROP COLUMN unit_name;
ALTER TABLE purchase_receipt_items DROP COLUMN medicine_unit_id;
