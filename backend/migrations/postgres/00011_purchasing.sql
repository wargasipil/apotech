-- +goose Up
-- Per-year sequence counters for human-friendly PO and receipt numbers.
CREATE TABLE po_no_counters (
  year     INT PRIMARY KEY,
  last_seq INT NOT NULL DEFAULT 0
);
CREATE TABLE rcv_no_counters (
  year     INT PRIMARY KEY,
  last_seq INT NOT NULL DEFAULT 0
);

CREATE TABLE purchase_orders (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  po_no         TEXT UNIQUE,                      -- assigned on create
  supplier_id   UUID NOT NULL REFERENCES suppliers(id),
  status        TEXT NOT NULL DEFAULT 'DRAFT'
                CHECK (status IN ('DRAFT','SENT','PARTIALLY_RECEIVED','RECEIVED','CLOSED','VOIDED')),
  expected_at   DATE,
  note          TEXT NOT NULL DEFAULT '',
  ordered_total BIGINT NOT NULL DEFAULT 0,
  paid_amount   BIGINT NOT NULL DEFAULT 0,
  created_by    UUID NOT NULL REFERENCES users(id),
  branch_id     UUID,                              -- placeholder for multi-branch
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  sent_at       TIMESTAMPTZ,
  closed_at     TIMESTAMPTZ
);
CREATE INDEX purchase_orders_supplier_idx ON purchase_orders(supplier_id);
CREATE INDEX purchase_orders_status_idx   ON purchase_orders(status);
CREATE INDEX purchase_orders_created_idx  ON purchase_orders(created_at);

CREATE TABLE purchase_order_items (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
  medicine_id       UUID NOT NULL REFERENCES medicines(id),
  ordered_qty       INTEGER NOT NULL CHECK (ordered_qty > 0),
  received_qty      INTEGER NOT NULL DEFAULT 0,
  unit_cost_price   BIGINT NOT NULL DEFAULT 0,
  subtotal          BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX purchase_order_items_po_idx        ON purchase_order_items(purchase_order_id);
CREATE INDEX purchase_order_items_medicine_idx  ON purchase_order_items(medicine_id);

CREATE TABLE purchase_receipts (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  receipt_no        TEXT UNIQUE,
  purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id),
  received_at       DATE NOT NULL DEFAULT CURRENT_DATE,
  received_by       UUID NOT NULL REFERENCES users(id),
  note              TEXT NOT NULL DEFAULT '',
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX purchase_receipts_po_idx ON purchase_receipts(purchase_order_id);

CREATE TABLE purchase_receipt_items (
  id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  purchase_receipt_id    UUID NOT NULL REFERENCES purchase_receipts(id) ON DELETE CASCADE,
  purchase_order_item_id UUID NOT NULL REFERENCES purchase_order_items(id),
  medicine_id            UUID NOT NULL REFERENCES medicines(id),
  qty                    INTEGER NOT NULL CHECK (qty > 0),
  unit_cost_price        BIGINT NOT NULL DEFAULT 0,
  batch_number           TEXT NOT NULL DEFAULT '',
  expiry_date            DATE NOT NULL,
  batch_id               UUID REFERENCES batches(id),  -- populated after batch row is created
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX purchase_receipt_items_receipt_idx ON purchase_receipt_items(purchase_receipt_id);
CREATE INDEX purchase_receipt_items_poi_idx     ON purchase_receipt_items(purchase_order_item_id);
CREATE INDEX purchase_receipt_items_batch_idx   ON purchase_receipt_items(batch_id) WHERE batch_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS purchase_receipt_items;
DROP TABLE IF EXISTS purchase_receipts;
DROP TABLE IF EXISTS purchase_order_items;
DROP TABLE IF EXISTS purchase_orders;
DROP TABLE IF EXISTS rcv_no_counters;
DROP TABLE IF EXISTS po_no_counters;
