-- +goose Up
CREATE TABLE batches (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  medicine_id  UUID NOT NULL REFERENCES medicines(id),
  supplier_id  UUID REFERENCES suppliers(id),
  batch_number TEXT NOT NULL DEFAULT '',
  expiry_date  DATE NOT NULL,
  cost_price   BIGINT NOT NULL DEFAULT 0,
  received_at  DATE NOT NULL DEFAULT CURRENT_DATE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX batches_medicine_idx ON batches(medicine_id);
CREATE INDEX batches_expiry_idx  ON batches(expiry_date);

-- +goose Down
DROP TABLE batches;
