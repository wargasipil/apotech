-- +goose Up
-- SQLite can't ALTER COLUMN SET NOT NULL nor ADD CONSTRAINT after the fact.
-- The portable build is fresh-DB only, so we add the column as nullable +
-- referencing warehouses(id) at declaration time; the service layer guards
-- non-null at write time (resolveWarehouse).
ALTER TABLE purchase_orders ADD COLUMN warehouse_id TEXT REFERENCES warehouses(id);
CREATE INDEX purchase_orders_warehouse_idx ON purchase_orders(warehouse_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS purchase_orders_warehouse_idx;
