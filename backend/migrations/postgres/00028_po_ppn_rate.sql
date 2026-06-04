-- +goose Up
-- Variable PPN rate per PO. 11% is the current Indonesian rate; future rate
-- transitions (11→12, sector-specific, exempt) are captured per-invoice.
ALTER TABLE purchase_orders
  ADD COLUMN ppn_rate INTEGER NOT NULL DEFAULT 11;

-- Legacy rows: those with ppn_enabled=true were computed at the fixed 11%;
-- keep them readable by leaving the new column at 11. ppn_amount already
-- carries the materialized value, so no recompute needed.

-- +goose Down
ALTER TABLE purchase_orders DROP COLUMN IF EXISTS ppn_rate;
