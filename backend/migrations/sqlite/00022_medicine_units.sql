-- +goose Up
-- Units of measure per medicine (box / strip / tablet, or bottle / tube, …).
-- Stock is always stored in the smallest "base" unit on the stock_movements
-- ledger; these rows define how larger units convert to base (factor) and the
-- independent sell price per unit.
CREATE TABLE medicine_units (
  id          TEXT PRIMARY KEY,
  medicine_id TEXT NOT NULL REFERENCES medicines(id),
  name        TEXT NOT NULL,
  factor      INTEGER NOT NULL CHECK (factor > 0),   -- base units per 1 of this unit (base = 1)
  is_base     BOOLEAN NOT NULL DEFAULT FALSE,
  sell_price  INTEGER NOT NULL DEFAULT 0,            -- minor currency, independent per unit
  sellable    BOOLEAN NOT NULL DEFAULT TRUE,
  purchasable BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order  INTEGER NOT NULL DEFAULT 0,
  active      BOOLEAN NOT NULL DEFAULT TRUE,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX medicine_units_name_idx ON medicine_units(medicine_id, name) WHERE active;
-- Exactly one base unit per medicine.
CREATE UNIQUE INDEX medicine_units_base_idx ON medicine_units(medicine_id) WHERE is_base;
CREATE INDEX medicine_units_medicine_idx ON medicine_units(medicine_id);

-- Backfill: the existing single `unit` becomes each medicine's base unit
-- (factor 1). No-op on a fresh DB; the explicit randomblob id covers a populated
-- run (TEXT PK has no DB default).
INSERT INTO medicine_units (id, medicine_id, name, factor, is_base, sell_price, sellable, purchasable, sort_order)
SELECT lower(hex(randomblob(16))), id, unit, 1, TRUE, unit_price, TRUE, TRUE, 0
FROM medicines;

-- Sale lines record the selling unit + the base-unit quantity consumed (one
-- ADD COLUMN per statement — SQLite has no multi-column ALTER).
ALTER TABLE sale_items ADD COLUMN medicine_unit_id TEXT;
ALTER TABLE sale_items ADD COLUMN unit_name TEXT NOT NULL DEFAULT '';
ALTER TABLE sale_items ADD COLUMN unit_factor INTEGER NOT NULL DEFAULT 1;
ALTER TABLE sale_items ADD COLUMN base_qty INTEGER NOT NULL DEFAULT 0;

UPDATE sale_items SET base_qty = qty WHERE base_qty = 0;
UPDATE sale_items SET
  medicine_unit_id = (SELECT mu.id FROM medicine_units mu WHERE mu.medicine_id = sale_items.medicine_id AND mu.is_base),
  unit_name        = (SELECT mu.name FROM medicine_units mu WHERE mu.medicine_id = sale_items.medicine_id AND mu.is_base)
WHERE medicine_unit_id IS NULL;

-- +goose Down
ALTER TABLE sale_items DROP COLUMN base_qty;
ALTER TABLE sale_items DROP COLUMN unit_factor;
ALTER TABLE sale_items DROP COLUMN unit_name;
ALTER TABLE sale_items DROP COLUMN medicine_unit_id;
DROP TABLE IF EXISTS medicine_units;
