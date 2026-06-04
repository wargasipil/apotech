-- +goose Up
-- Multi-warehouse (gudang) support. Warehouses replace branches as the stock
-- location concept: stock becomes per-warehouse and movable between warehouses.
-- Branch tables are left dormant (not dropped) for a later cleanup migration.

CREATE TABLE warehouses (
  id         TEXT PRIMARY KEY,
  code       TEXT UNIQUE NOT NULL,            -- short slug, e.g. "MAIN", "GD02"
  name       TEXT NOT NULL,
  address    TEXT NOT NULL DEFAULT '',
  phone      TEXT NOT NULL DEFAULT '',
  is_default BOOLEAN NOT NULL DEFAULT FALSE,  -- where legacy stock + receives land
  active     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- At most one default warehouse globally.
CREATE UNIQUE INDEX warehouses_default_idx ON warehouses(is_default) WHERE is_default;

CREATE TABLE user_warehouses (
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  warehouse_id TEXT NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
  is_default   BOOLEAN NOT NULL DEFAULT FALSE,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, warehouse_id)
);
CREATE INDEX user_warehouses_warehouse_idx ON user_warehouses(warehouse_id);
-- One default warehouse per user.
CREATE UNIQUE INDEX user_warehouses_default_idx ON user_warehouses(user_id) WHERE is_default;

-- Seed the default warehouse and grant every existing user access to it.
-- TEXT primary keys have no DB-side default (UUIDs are generated Go-side), so
-- this raw seed provides an explicit id.
INSERT INTO warehouses (id, code, name, is_default) VALUES ('00000000-0000-4000-8000-0000000000a1', 'MAIN', 'Gudang Utama', TRUE);
INSERT INTO user_warehouses (user_id, warehouse_id, is_default)
SELECT u.id, w.id, TRUE
FROM users u, warehouses w
WHERE w.code = 'MAIN';

-- Stock transfers (mutasi stok antar gudang). Each transfer line becomes a
-- TRANSFER_OUT (source) + TRANSFER_IN (dest) movement pair linked via transfer_id.
CREATE TABLE transfer_no_counters (
  year     INTEGER PRIMARY KEY,
  last_seq INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE stock_transfers (
  id                TEXT PRIMARY KEY,
  transfer_no       TEXT UNIQUE,                    -- TRF-YYYY-NNNN
  from_warehouse_id TEXT NOT NULL REFERENCES warehouses(id),
  to_warehouse_id   TEXT NOT NULL REFERENCES warehouses(id),
  note              TEXT NOT NULL DEFAULT '',
  created_by        TEXT NOT NULL REFERENCES users(id),
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX stock_transfers_created_idx ON stock_transfers(created_at DESC);

-- Warehouse dimension: the ledger gets it, plus the selling + counting surfaces
-- so their movements stay consistent. SQLite cannot ALTER a column to NOT NULL,
-- so warehouse_id is added nullable here (vs Postgres, which backfills then
-- locks NOT NULL below). The app always stamps warehouse_id via resolveWarehouse,
-- so no NULL rows ever exist — the constraint is enforced in application code.
ALTER TABLE stock_movements    ADD COLUMN warehouse_id TEXT REFERENCES warehouses(id);
ALTER TABLE stock_movements    ADD COLUMN transfer_id  TEXT REFERENCES stock_transfers(id);
ALTER TABLE sales              ADD COLUMN warehouse_id TEXT REFERENCES warehouses(id);
ALTER TABLE stocktake_sessions ADD COLUMN warehouse_id TEXT REFERENCES warehouses(id);

-- Backfill everything to the default warehouse (no-op on a fresh DB).
UPDATE stock_movements    SET warehouse_id = (SELECT id FROM warehouses WHERE code = 'MAIN') WHERE warehouse_id IS NULL;
UPDATE sales              SET warehouse_id = (SELECT id FROM warehouses WHERE code = 'MAIN') WHERE warehouse_id IS NULL;
UPDATE stocktake_sessions SET warehouse_id = (SELECT id FROM warehouses WHERE code = 'MAIN') WHERE warehouse_id IS NULL;
-- Postgres here runs: ALTER TABLE stock_movements ALTER COLUMN warehouse_id SET NOT NULL;
-- SQLite cannot alter a column to NOT NULL; see the note above (app-enforced).

CREATE INDEX stock_movements_warehouse_idx ON stock_movements(batch_id, warehouse_id);
CREATE INDEX stock_movements_transfer_idx  ON stock_movements(transfer_id) WHERE transfer_id IS NOT NULL;
CREATE INDEX sales_warehouse_idx ON sales(warehouse_id);

-- Postgres widens the stock_movements type CHECK here (DROP/ADD CONSTRAINT) to
-- allow TRANSFER_IN/TRANSFER_OUT. SQLite cannot ALTER constraints; the widened
-- set is baked into 00007's CREATE, so this step is a no-op on SQLite.

-- +goose Down
DROP INDEX IF EXISTS sales_warehouse_idx;
DROP INDEX IF EXISTS stock_movements_transfer_idx;
DROP INDEX IF EXISTS stock_movements_warehouse_idx;
ALTER TABLE stocktake_sessions DROP COLUMN warehouse_id;
ALTER TABLE sales              DROP COLUMN warehouse_id;
ALTER TABLE stock_movements    DROP COLUMN transfer_id;
ALTER TABLE stock_movements    DROP COLUMN warehouse_id;
DROP TABLE IF EXISTS stock_transfers;
DROP TABLE IF EXISTS transfer_no_counters;
DROP TABLE IF EXISTS user_warehouses;
DROP TABLE IF EXISTS warehouses;
