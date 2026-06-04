-- +goose Up
-- Multi-warehouse (gudang) support. Warehouses replace branches as the stock
-- location concept: stock becomes per-warehouse and movable between warehouses.
-- Branch tables are left dormant (not dropped) for a later cleanup migration.

CREATE TABLE warehouses (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code       TEXT UNIQUE NOT NULL,            -- short slug, e.g. "MAIN", "GD02"
  name       TEXT NOT NULL,
  address    TEXT NOT NULL DEFAULT '',
  phone      TEXT NOT NULL DEFAULT '',
  is_default BOOLEAN NOT NULL DEFAULT FALSE,  -- where legacy stock + receives land
  active     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- At most one default warehouse globally.
CREATE UNIQUE INDEX warehouses_default_idx ON warehouses(is_default) WHERE is_default;

CREATE TABLE user_warehouses (
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
  is_default   BOOLEAN NOT NULL DEFAULT FALSE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, warehouse_id)
);
CREATE INDEX user_warehouses_warehouse_idx ON user_warehouses(warehouse_id);
-- One default warehouse per user.
CREATE UNIQUE INDEX user_warehouses_default_idx ON user_warehouses(user_id) WHERE is_default;

-- Seed the default warehouse and grant every existing user access to it.
INSERT INTO warehouses (code, name, is_default) VALUES ('MAIN', 'Gudang Utama', TRUE);
INSERT INTO user_warehouses (user_id, warehouse_id, is_default)
SELECT u.id, w.id, TRUE
FROM users u, warehouses w
WHERE w.code = 'MAIN';

-- Stock transfers (mutasi stok antar gudang). Each transfer line becomes a
-- TRANSFER_OUT (source) + TRANSFER_IN (dest) movement pair linked via transfer_id.
CREATE TABLE transfer_no_counters (
  year     INT PRIMARY KEY,
  last_seq INT NOT NULL DEFAULT 0
);

CREATE TABLE stock_transfers (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  transfer_no       TEXT UNIQUE,                    -- TRF-YYYY-NNNN
  from_warehouse_id UUID NOT NULL REFERENCES warehouses(id),
  to_warehouse_id   UUID NOT NULL REFERENCES warehouses(id),
  note              TEXT NOT NULL DEFAULT '',
  created_by        UUID NOT NULL REFERENCES users(id),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX stock_transfers_created_idx ON stock_transfers(created_at DESC);

-- Warehouse dimension: the ledger gets it (mandatory), plus the selling +
-- counting surfaces so their movements stay consistent.
ALTER TABLE stock_movements    ADD COLUMN warehouse_id UUID REFERENCES warehouses(id);
ALTER TABLE stock_movements    ADD COLUMN transfer_id  UUID REFERENCES stock_transfers(id);
ALTER TABLE sales              ADD COLUMN warehouse_id UUID REFERENCES warehouses(id);
ALTER TABLE stocktake_sessions ADD COLUMN warehouse_id UUID REFERENCES warehouses(id);

-- Backfill everything to the default warehouse, then lock the ledger NOT NULL
-- (per-warehouse SUM breaks if any movement has a NULL warehouse).
UPDATE stock_movements    SET warehouse_id = (SELECT id FROM warehouses WHERE code = 'MAIN') WHERE warehouse_id IS NULL;
UPDATE sales              SET warehouse_id = (SELECT id FROM warehouses WHERE code = 'MAIN') WHERE warehouse_id IS NULL;
UPDATE stocktake_sessions SET warehouse_id = (SELECT id FROM warehouses WHERE code = 'MAIN') WHERE warehouse_id IS NULL;
ALTER TABLE stock_movements ALTER COLUMN warehouse_id SET NOT NULL;

CREATE INDEX stock_movements_warehouse_idx ON stock_movements(batch_id, warehouse_id);
CREATE INDEX stock_movements_transfer_idx  ON stock_movements(transfer_id) WHERE transfer_id IS NOT NULL;
CREATE INDEX sales_warehouse_idx ON sales(warehouse_id);

-- Allow the two transfer movement types.
ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_type_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_type_check
  CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF','TRANSFER_IN','TRANSFER_OUT'));

-- +goose Down
ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_type_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_type_check
  CHECK (type IN ('PURCHASE','SALE','ADJUSTMENT','WRITE_OFF'));
DROP INDEX IF EXISTS sales_warehouse_idx;
DROP INDEX IF EXISTS stock_movements_transfer_idx;
DROP INDEX IF EXISTS stock_movements_warehouse_idx;
ALTER TABLE stocktake_sessions DROP COLUMN IF EXISTS warehouse_id;
ALTER TABLE sales              DROP COLUMN IF EXISTS warehouse_id;
ALTER TABLE stock_movements    DROP COLUMN IF EXISTS transfer_id;
ALTER TABLE stock_movements    DROP COLUMN IF EXISTS warehouse_id;
DROP TABLE IF EXISTS stock_transfers;
DROP TABLE IF EXISTS transfer_no_counters;
DROP TABLE IF EXISTS user_warehouses;
DROP TABLE IF EXISTS warehouses;
