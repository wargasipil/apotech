-- +goose Up
CREATE TABLE batches (
  id           TEXT PRIMARY KEY,
  medicine_id  TEXT NOT NULL REFERENCES medicines(id),
  supplier_id  TEXT REFERENCES suppliers(id),
  batch_number TEXT NOT NULL DEFAULT '',
  expiry_date  DATE NOT NULL,
  cost_price   INTEGER NOT NULL DEFAULT 0,
  received_at  DATE NOT NULL DEFAULT CURRENT_DATE,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX batches_medicine_idx ON batches(medicine_id);
CREATE INDEX batches_expiry_idx  ON batches(expiry_date);

-- +goose Down
DROP TABLE batches;
