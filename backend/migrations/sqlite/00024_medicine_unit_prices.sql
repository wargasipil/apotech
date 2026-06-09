-- +goose Up
-- Fresh-DB on portable: backfill is a no-op. Service layer seeds a price row
-- via recordUnitPrice whenever a unit is created.
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

-- +goose Down
DROP TABLE medicine_unit_prices;
