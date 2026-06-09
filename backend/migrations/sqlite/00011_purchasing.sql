-- +goose Up
CREATE TABLE po_no_counters (
  year     INTEGER PRIMARY KEY,
  last_seq INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE rcv_no_counters (
  year     INTEGER PRIMARY KEY,
  last_seq INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE purchase_orders (
  id            TEXT PRIMARY KEY,
  po_no         TEXT UNIQUE,
  supplier_id   TEXT NOT NULL REFERENCES suppliers(id),
  status        TEXT NOT NULL DEFAULT 'DRAFT'
                CHECK (status IN ('DRAFT','SENT','PARTIALLY_RECEIVED','RECEIVED','CLOSED','VOIDED')),
  expected_at   DATE,
  note          TEXT NOT NULL DEFAULT '',
  ordered_total INTEGER NOT NULL DEFAULT 0,
  paid_amount   INTEGER NOT NULL DEFAULT 0,
  created_by    TEXT NOT NULL REFERENCES users(id),
  branch_id     TEXT,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  sent_at       DATETIME,
  closed_at     DATETIME
);
CREATE INDEX purchase_orders_supplier_idx ON purchase_orders(supplier_id);
CREATE INDEX purchase_orders_status_idx   ON purchase_orders(status);
CREATE INDEX purchase_orders_created_idx  ON purchase_orders(created_at);

CREATE TABLE purchase_order_items (
  id                TEXT PRIMARY KEY,
  purchase_order_id TEXT NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
  medicine_id       TEXT NOT NULL REFERENCES medicines(id),
  ordered_qty       INTEGER NOT NULL CHECK (ordered_qty > 0),
  received_qty      INTEGER NOT NULL DEFAULT 0,
  unit_cost_price   INTEGER NOT NULL DEFAULT 0,
  subtotal          INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX purchase_order_items_po_idx        ON purchase_order_items(purchase_order_id);
CREATE INDEX purchase_order_items_medicine_idx  ON purchase_order_items(medicine_id);

CREATE TABLE purchase_receipts (
  id                TEXT PRIMARY KEY,
  receipt_no        TEXT UNIQUE,
  purchase_order_id TEXT NOT NULL REFERENCES purchase_orders(id),
  received_at       DATE NOT NULL DEFAULT CURRENT_DATE,
  received_by       TEXT NOT NULL REFERENCES users(id),
  note              TEXT NOT NULL DEFAULT '',
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX purchase_receipts_po_idx ON purchase_receipts(purchase_order_id);

CREATE TABLE purchase_receipt_items (
  id                     TEXT PRIMARY KEY,
  purchase_receipt_id    TEXT NOT NULL REFERENCES purchase_receipts(id) ON DELETE CASCADE,
  purchase_order_item_id TEXT NOT NULL REFERENCES purchase_order_items(id),
  medicine_id            TEXT NOT NULL REFERENCES medicines(id),
  qty                    INTEGER NOT NULL CHECK (qty > 0),
  unit_cost_price        INTEGER NOT NULL DEFAULT 0,
  batch_number           TEXT NOT NULL DEFAULT '',
  expiry_date            DATE NOT NULL,
  batch_id               TEXT REFERENCES batches(id),
  created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
