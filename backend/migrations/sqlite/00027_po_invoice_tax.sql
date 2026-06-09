-- +goose Up
-- SQLite supports RENAME COLUMN since 3.25 (we ship modernc.org/sqlite 1.52+
-- = SQLite 3.45+). One ADD COLUMN per ALTER TABLE statement.
ALTER TABLE purchase_orders RENAME COLUMN expected_at TO invoice_date;
ALTER TABLE purchase_orders ADD COLUMN invoice_no    TEXT    NOT NULL DEFAULT '';
ALTER TABLE purchase_orders ADD COLUMN due_at        DATE;
ALTER TABLE purchase_orders ADD COLUMN subtotal      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE purchase_orders ADD COLUMN cart_discount INTEGER NOT NULL DEFAULT 0;
ALTER TABLE purchase_orders ADD COLUMN ppn_enabled   BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE purchase_orders ADD COLUMN ppn_amount    INTEGER NOT NULL DEFAULT 0;

-- Fresh-DB on portable: no rows to fix.

-- +goose Down
ALTER TABLE purchase_orders RENAME COLUMN invoice_date TO expected_at;
