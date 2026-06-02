-- +goose Up
-- Capture supplier invoice metadata + PPN/discount math at PO create time.
ALTER TABLE purchase_orders RENAME COLUMN expected_at TO invoice_date;

ALTER TABLE purchase_orders
  ADD COLUMN invoice_no    TEXT     NOT NULL DEFAULT '',
  ADD COLUMN due_at        DATE     NULL,
  ADD COLUMN subtotal      BIGINT   NOT NULL DEFAULT 0,
  ADD COLUMN cart_discount BIGINT   NOT NULL DEFAULT 0,
  ADD COLUMN ppn_enabled   BOOLEAN  NOT NULL DEFAULT FALSE,
  ADD COLUMN ppn_amount    BIGINT   NOT NULL DEFAULT 0;

-- Existing rows: subtotal == ordered_total; discount/PPN are zero. Keeps
-- ordered_total = subtotal − discount + ppn invariant true for legacy data.
UPDATE purchase_orders SET subtotal = ordered_total WHERE subtotal = 0;

-- +goose Down
ALTER TABLE purchase_orders
  DROP COLUMN IF EXISTS ppn_amount,
  DROP COLUMN IF EXISTS ppn_enabled,
  DROP COLUMN IF EXISTS cart_discount,
  DROP COLUMN IF EXISTS subtotal,
  DROP COLUMN IF EXISTS due_at,
  DROP COLUMN IF EXISTS invoice_no;
ALTER TABLE purchase_orders RENAME COLUMN invoice_date TO expected_at;
