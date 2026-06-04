-- +goose Up
-- Add warehouse_id to purchase_orders so POs are first-class warehouse documents.
-- SQLite cannot ALTER a column to NOT NULL nor ADD a named FK constraint after
-- the fact, so warehouse_id is added nullable with the FK inline (vs Postgres,
-- which backfills, then SET NOT NULL, then ADD CONSTRAINT). CreatePurchaseOrder
-- always stamps warehouse_id, so the invariant holds in application code.
ALTER TABLE purchase_orders ADD COLUMN warehouse_id TEXT REFERENCES warehouses(id);

-- Backfill all existing rows to the global default warehouse (no-op on a fresh DB).
UPDATE purchase_orders
SET warehouse_id = (SELECT id FROM warehouses WHERE is_default LIMIT 1)
WHERE warehouse_id IS NULL;

CREATE INDEX purchase_orders_warehouse_idx ON purchase_orders(warehouse_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS purchase_orders_warehouse_idx;
ALTER TABLE purchase_orders DROP COLUMN warehouse_id;
