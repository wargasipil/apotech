-- +goose Up
-- Capture the supplier invoice number (nomor faktur) on each delivery receipt.
ALTER TABLE purchase_receipts ADD COLUMN invoice_no TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE purchase_receipts DROP COLUMN IF EXISTS invoice_no;
