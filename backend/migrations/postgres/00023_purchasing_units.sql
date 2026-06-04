-- +goose Up
-- Phase 2 buy-in-units: PO + receipt lines remember the purchasable unit they
-- were entered in. Quantities (ordered_qty / received_qty / qty) stay in BASE
-- units; these columns are display/entry metadata (unit_factor = base per 1).
ALTER TABLE purchase_order_items
  ADD COLUMN medicine_unit_id UUID,
  ADD COLUMN unit_name TEXT NOT NULL DEFAULT '',
  ADD COLUMN unit_factor BIGINT NOT NULL DEFAULT 1;

ALTER TABLE purchase_receipt_items
  ADD COLUMN medicine_unit_id UUID,
  ADD COLUMN unit_name TEXT NOT NULL DEFAULT '',
  ADD COLUMN unit_factor BIGINT NOT NULL DEFAULT 1;

-- Backfill existing rows to each medicine's base unit (factor 1); quantities are
-- already base, so no qty change is needed.
UPDATE purchase_order_items poi SET medicine_unit_id = mu.id, unit_name = mu.name
FROM medicine_units mu
WHERE mu.medicine_id = poi.medicine_id AND mu.is_base AND poi.medicine_unit_id IS NULL;

UPDATE purchase_receipt_items pri SET medicine_unit_id = mu.id, unit_name = mu.name
FROM medicine_units mu
WHERE mu.medicine_id = pri.medicine_id AND mu.is_base AND pri.medicine_unit_id IS NULL;

-- +goose Down
ALTER TABLE purchase_order_items
  DROP COLUMN IF EXISTS medicine_unit_id,
  DROP COLUMN IF EXISTS unit_name,
  DROP COLUMN IF EXISTS unit_factor;
ALTER TABLE purchase_receipt_items
  DROP COLUMN IF EXISTS medicine_unit_id,
  DROP COLUMN IF EXISTS unit_name,
  DROP COLUMN IF EXISTS unit_factor;
