-- +goose Up
-- Per-unit sell-price history, mirroring medicine_prices but keyed by the unit.
-- Exactly one open row (effective_to IS NULL) per unit. changed_by is nullable so
-- the backfill (system-seeded baseline) can insert without a user.
CREATE TABLE medicine_unit_prices (
  id               TEXT PRIMARY KEY,
  medicine_unit_id TEXT NOT NULL REFERENCES medicine_units(id),
  unit_sell_price  INTEGER NOT NULL,
  effective_from   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  effective_to     DATETIME,
  changed_by       TEXT REFERENCES users(id),
  created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX medicine_unit_prices_open_idx
  ON medicine_unit_prices(medicine_unit_id)
  WHERE effective_to IS NULL;
CREATE INDEX medicine_unit_prices_unit_idx ON medicine_unit_prices(medicine_unit_id);

-- Backfill: one open row per existing unit at its current sell_price. No-op on a
-- fresh DB; the explicit randomblob id covers a populated run (TEXT PK).
INSERT INTO medicine_unit_prices (id, medicine_unit_id, unit_sell_price, effective_from)
SELECT lower(hex(randomblob(16))), id, sell_price, created_at FROM medicine_units;

-- +goose Down
DROP TABLE medicine_unit_prices;
