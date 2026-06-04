-- +goose Up
-- Capture supplier invoice metadata + PPN/discount math at PO create time.
ALTER TABLE purchase_orders RENAME COLUMN expected_at TO invoice_date;

-- One ADD COLUMN per statement (SQLite has no multi-column ALTER).
ALTER TABLE purchase_orders ADD COLUMN invoice_no    TEXT     NOT NULL DEFAULT '';
ALTER TABLE purchase_orders ADD COLUMN due_at        DATE;
ALTER TABLE purchase_orders ADD COLUMN subtotal      INTEGER  NOT NULL DEFAULT 0;
ALTER TABLE purchase_orders ADD COLUMN cart_discount INTEGER  NOT NULL DEFAULT 0;
ALTER TABLE purchase_orders ADD COLUMN ppn_enabled   BOOLEAN  NOT NULL DEFAULT FALSE;
ALTER TABLE purchase_orders ADD COLUMN ppn_amount    INTEGER  NOT NULL DEFAULT 0;

-- Existing rows: subtotal == ordered_total; discount/PPN are zero. Keeps
-- ordered_total = subtotal − discount + ppn invariant true for legacy data.
UPDATE purchase_orders SET subtotal = ordered_total WHERE subtotal = 0;

-- +goose Down
ALTER TABLE purchase_orders DROP COLUMN ppn_amount;
ALTER TABLE purchase_orders DROP COLUMN ppn_enabled;
ALTER TABLE purchase_orders DROP COLUMN cart_discount;
ALTER TABLE purchase_orders DROP COLUMN subtotal;
ALTER TABLE purchase_orders DROP COLUMN due_at;
ALTER TABLE purchase_orders DROP COLUMN invoice_no;
ALTER TABLE purchase_orders RENAME COLUMN invoice_date TO expected_at;
