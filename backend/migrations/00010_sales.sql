-- +goose Up
-- Per-year sale numbering: a small table tracks the last number for each year.
CREATE TABLE sale_no_counters (
  year       INT  PRIMARY KEY,
  last_seq   INT  NOT NULL DEFAULT 0
);

CREATE TABLE sales (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_no         TEXT UNIQUE,                 -- populated on CompleteSale
  customer_id     UUID REFERENCES customers(id),
  cashier_user_id UUID NOT NULL REFERENCES users(id),
  payment_source  TEXT,                        -- CASH | BPJS | INSURANCE_OTHER (set on Complete)
  tax_invoice_no  TEXT,
  subtotal        BIGINT NOT NULL DEFAULT 0,   -- sum of line_totals (before cart discount)
  cart_discount   BIGINT NOT NULL DEFAULT 0,
  total           BIGINT NOT NULL DEFAULT 0,   -- subtotal - cart_discount
  paid_amount     BIGINT NOT NULL DEFAULT 0,
  status          TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','COMPLETED','VOIDED')),
  branch_id       UUID,                        -- placeholder for multi-branch phase
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at    TIMESTAMPTZ
);
CREATE INDEX sales_created_idx     ON sales(created_at);
CREATE INDEX sales_status_idx      ON sales(status);
CREATE INDEX sales_completed_idx   ON sales(completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX sales_customer_idx    ON sales(customer_id) WHERE customer_id IS NOT NULL;

CREATE TABLE sale_items (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id              UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
  medicine_id          UUID NOT NULL REFERENCES medicines(id),
  batch_id             UUID REFERENCES batches(id),    -- assigned on CompleteSale via FEFO
  qty                  INTEGER NOT NULL CHECK (qty > 0),
  unit_price_snapshot  BIGINT NOT NULL DEFAULT 0,      -- snapshot on add; finalized on Complete
  line_discount        BIGINT NOT NULL DEFAULT 0,
  line_total           BIGINT NOT NULL DEFAULT 0,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sale_items_sale_idx     ON sale_items(sale_id);
CREATE INDEX sale_items_medicine_idx ON sale_items(medicine_id);

-- Stock-movement audit link back to the sale_item that caused a SALE movement.
ALTER TABLE stock_movements ADD COLUMN sale_item_id UUID REFERENCES sale_items(id);
CREATE INDEX stock_movements_sale_item_idx ON stock_movements(sale_item_id) WHERE sale_item_id IS NOT NULL;

-- branch_id placeholder columns on operational tables (for the multi-branch phase).
ALTER TABLE sale_items     ADD COLUMN branch_id UUID;
ALTER TABLE stock_movements ADD COLUMN branch_id UUID;

-- +goose Down
ALTER TABLE stock_movements DROP COLUMN IF EXISTS branch_id;
ALTER TABLE sale_items     DROP COLUMN IF EXISTS branch_id;
DROP INDEX IF EXISTS stock_movements_sale_item_idx;
ALTER TABLE stock_movements DROP COLUMN IF EXISTS sale_item_id;
DROP TABLE IF EXISTS sale_items;
DROP TABLE IF EXISTS sales;
DROP TABLE IF EXISTS sale_no_counters;
