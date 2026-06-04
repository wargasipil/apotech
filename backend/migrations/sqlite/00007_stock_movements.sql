-- +goose Up
CREATE TABLE stock_movements (
  id         TEXT PRIMARY KEY,
  batch_id   TEXT NOT NULL REFERENCES batches(id),
  qty        INTEGER NOT NULL CHECK (qty <> 0),
  -- The type CHECK is pre-widened here with TRANSFER_IN/TRANSFER_OUT. On
  -- Postgres these are added later by 00019 via ALTER ... DROP/ADD CONSTRAINT,
  -- which SQLite cannot do; baking the final set in at create time yields the
  -- identical end-state schema (00019 is a no-op for this CHECK on SQLite).
  type       TEXT NOT NULL CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF','TRANSFER_IN','TRANSFER_OUT')),
  reason     TEXT NOT NULL DEFAULT '',
  user_id    TEXT NOT NULL REFERENCES users(id),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX stock_movements_batch_idx   ON stock_movements(batch_id);
CREATE INDEX stock_movements_created_idx ON stock_movements(created_at);

-- +goose Down
DROP TABLE stock_movements;
