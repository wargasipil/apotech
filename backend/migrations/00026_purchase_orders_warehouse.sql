-- +goose Up
-- Add warehouse_id to purchase_orders so POs are first-class warehouse documents.
-- Backfill all existing rows to the global default warehouse, then enforce NOT NULL + FK.
ALTER TABLE purchase_orders ADD COLUMN warehouse_id UUID;

UPDATE purchase_orders
SET warehouse_id = (SELECT id FROM warehouses WHERE is_default LIMIT 1)
WHERE warehouse_id IS NULL;

ALTER TABLE purchase_orders ALTER COLUMN warehouse_id SET NOT NULL;
ALTER TABLE purchase_orders
  ADD CONSTRAINT purchase_orders_warehouse_fk
  FOREIGN KEY (warehouse_id) REFERENCES warehouses(id);

CREATE INDEX purchase_orders_warehouse_idx ON purchase_orders(warehouse_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS purchase_orders_warehouse_idx;
ALTER TABLE purchase_orders DROP CONSTRAINT IF EXISTS purchase_orders_warehouse_fk;
ALTER TABLE purchase_orders DROP COLUMN IF EXISTS warehouse_id;
