-- +goose Up
CREATE TABLE stock_movements (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  batch_id   UUID NOT NULL REFERENCES batches(id),
  qty        INTEGER NOT NULL CHECK (qty <> 0),
  type       TEXT NOT NULL CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF')),
  reason     TEXT NOT NULL DEFAULT '',
  user_id    UUID NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX stock_movements_batch_idx   ON stock_movements(batch_id);
CREATE INDEX stock_movements_created_idx ON stock_movements(created_at);

-- +goose Down
DROP TABLE stock_movements;
