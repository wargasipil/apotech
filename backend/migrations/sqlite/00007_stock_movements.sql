-- +goose Up
-- SQLite note: include the TRANSFER_IN/TRANSFER_OUT enum values up front (the
-- Postgres set lands them via ALTER TABLE DROP/ADD CONSTRAINT in 00019, which
-- SQLite doesn't support). The schema is otherwise identical to the Postgres
-- 00007 + 00019 union.
CREATE TABLE stock_movements (
  id         TEXT PRIMARY KEY,
  batch_id   TEXT NOT NULL REFERENCES batches(id),
  qty        INTEGER NOT NULL CHECK (qty <> 0),
  type       TEXT NOT NULL CHECK (type IN
              ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF','TRANSFER_IN','TRANSFER_OUT')),
  reason     TEXT NOT NULL DEFAULT '',
  user_id    TEXT NOT NULL REFERENCES users(id),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX stock_movements_batch_idx   ON stock_movements(batch_id);
CREATE INDEX stock_movements_created_idx ON stock_movements(created_at);

-- +goose Down
DROP TABLE stock_movements;
