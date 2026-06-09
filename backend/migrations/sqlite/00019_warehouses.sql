-- +goose Up
-- Multi-warehouse (gudang) support. SQLite carve-outs vs Postgres 00019:
--   * No ALTER TABLE ... ALTER COLUMN ... SET NOT NULL — `warehouse_id`
--     stays nullable in the schema; the service layer (`resolveWarehouse`)
--     enforces population on every write. SQLite is fresh-DB only in the
--     portable flavor, so there are no legacy NULL rows to backfill.
--   * No ALTER TABLE ... DROP/ADD CONSTRAINT — the TRANSFER_IN/TRANSFER_OUT
--     enum values are baked into 00007 sqlite, so this migration doesn't
--     touch the type CHECK.

CREATE TABLE warehouses (
  id         TEXT PRIMARY KEY,
  code       TEXT UNIQUE NOT NULL,
  name       TEXT NOT NULL,
  address    TEXT NOT NULL DEFAULT '',
  phone      TEXT NOT NULL DEFAULT '',
  is_default BOOLEAN NOT NULL DEFAULT 0,
  active     BOOLEAN NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX warehouses_default_idx ON warehouses(is_default) WHERE is_default = 1;

CREATE TABLE user_warehouses (
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  warehouse_id TEXT NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
  is_default   BOOLEAN NOT NULL DEFAULT 0,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, warehouse_id)
);
CREATE INDEX user_warehouses_warehouse_idx ON user_warehouses(warehouse_id);
CREATE UNIQUE INDEX user_warehouses_default_idx ON user_warehouses(user_id) WHERE is_default = 1;

-- Seed the default warehouse with a fixed UUID so the service-layer auto-grant
-- (grantDefaultWarehouse) can target it by code on first run.
INSERT INTO warehouses (id, code, name, is_default)
  VALUES ('00000000-0000-0000-0000-00000000a001', 'MAIN', 'Gudang Utama', 1);

CREATE TABLE transfer_no_counters (
  year     INTEGER PRIMARY KEY,
  last_seq INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE stock_transfers (
  id                TEXT PRIMARY KEY,
  transfer_no       TEXT UNIQUE,
  from_warehouse_id TEXT NOT NULL REFERENCES warehouses(id),
  to_warehouse_id   TEXT NOT NULL REFERENCES warehouses(id),
  note              TEXT NOT NULL DEFAULT '',
  created_by        TEXT NOT NULL REFERENCES users(id),
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX stock_transfers_created_idx ON stock_transfers(created_at DESC);

ALTER TABLE stock_movements    ADD COLUMN warehouse_id TEXT REFERENCES warehouses(id);
ALTER TABLE stock_movements    ADD COLUMN transfer_id  TEXT REFERENCES stock_transfers(id);
ALTER TABLE sales              ADD COLUMN warehouse_id TEXT REFERENCES warehouses(id);
ALTER TABLE stocktake_sessions ADD COLUMN warehouse_id TEXT REFERENCES warehouses(id);

CREATE INDEX stock_movements_warehouse_idx ON stock_movements(batch_id, warehouse_id);
CREATE INDEX stock_movements_transfer_idx  ON stock_movements(transfer_id) WHERE transfer_id IS NOT NULL;
CREATE INDEX sales_warehouse_idx ON sales(warehouse_id);

-- +goose Down
DROP INDEX IF EXISTS sales_warehouse_idx;
DROP INDEX IF EXISTS stock_movements_transfer_idx;
DROP INDEX IF EXISTS stock_movements_warehouse_idx;
DROP TABLE IF EXISTS stock_transfers;
DROP TABLE IF EXISTS transfer_no_counters;
DROP TABLE IF EXISTS user_warehouses;
DROP TABLE IF EXISTS warehouses;
