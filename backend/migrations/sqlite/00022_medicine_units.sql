-- +goose Up
-- SQLite carve-out: the portable build is fresh-DB only, so the backfill of
-- medicine_units from existing medicines (and sale_items unit columns) is a
-- no-op and is omitted. The service layer creates a base unit on every
-- CreateMedicine.

CREATE TABLE medicine_units (
  id          TEXT PRIMARY KEY,
  medicine_id TEXT NOT NULL REFERENCES medicines(id),
  name        TEXT NOT NULL,
  factor      INTEGER NOT NULL CHECK (factor > 0),
  is_base     BOOLEAN NOT NULL DEFAULT 0,
  sell_price  INTEGER NOT NULL DEFAULT 0,
  sellable    BOOLEAN NOT NULL DEFAULT 1,
  purchasable BOOLEAN NOT NULL DEFAULT 1,
  sort_order  INTEGER NOT NULL DEFAULT 0,
  active      BOOLEAN NOT NULL DEFAULT 1,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX medicine_units_name_idx ON medicine_units(medicine_id, name) WHERE active = 1;
CREATE UNIQUE INDEX medicine_units_base_idx ON medicine_units(medicine_id) WHERE is_base = 1;
CREATE INDEX medicine_units_medicine_idx ON medicine_units(medicine_id);

-- Sale lines record the selling unit + the base-unit quantity consumed.
ALTER TABLE sale_items ADD COLUMN medicine_unit_id TEXT;
ALTER TABLE sale_items ADD COLUMN unit_name TEXT NOT NULL DEFAULT '';
ALTER TABLE sale_items ADD COLUMN unit_factor INTEGER NOT NULL DEFAULT 1;
ALTER TABLE sale_items ADD COLUMN base_qty INTEGER NOT NULL DEFAULT 0;

-- +goose Down
DROP TABLE IF EXISTS medicine_units;
