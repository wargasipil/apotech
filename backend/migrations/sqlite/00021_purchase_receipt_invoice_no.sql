-- +goose Up
ALTER TABLE purchase_receipts ADD COLUMN invoice_no TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite cannot DROP COLUMN before 3.35; leave it in place on downgrade.
SELECT 1;
