-- +goose Up
-- Units of measure per medicine (box / strip / tablet, or bottle / tube, …).
-- Stock is always stored in the smallest "base" unit on the stock_movements
-- ledger; these rows define how larger units convert to base (factor) and the
-- independent sell price per unit. Phase 1 = sell-in-units; `purchasable` is
-- reserved for Phase 2 (buy-in-units) and added now to avoid a later migration.
CREATE TABLE medicine_units (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  medicine_id UUID NOT NULL REFERENCES medicines(id),
  name        TEXT NOT NULL,
  factor      BIGINT NOT NULL CHECK (factor > 0),   -- base units per 1 of this unit (base = 1)
  is_base     BOOLEAN NOT NULL DEFAULT FALSE,
  sell_price  BIGINT NOT NULL DEFAULT 0,            -- minor currency, independent per unit
  sellable    BOOLEAN NOT NULL DEFAULT TRUE,
  purchasable BOOLEAN NOT NULL DEFAULT TRUE,        -- Phase 2
  sort_order  INT NOT NULL DEFAULT 0,
  active      BOOLEAN NOT NULL DEFAULT TRUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX medicine_units_name_idx ON medicine_units(medicine_id, name) WHERE active;
-- Exactly one base unit per medicine.
CREATE UNIQUE INDEX medicine_units_base_idx ON medicine_units(medicine_id) WHERE is_base;
CREATE INDEX medicine_units_medicine_idx ON medicine_units(medicine_id);

-- Backfill: the existing single `unit` becomes each medicine's base unit
-- (factor 1), priced at the current unit_price. Existing stock is already in
-- this unit, so it is already "base" — no ledger migration needed.
INSERT INTO medicine_units (medicine_id, name, factor, is_base, sell_price, sellable, purchasable, sort_order)
SELECT id, unit, 1, TRUE, unit_price, TRUE, TRUE, 0
FROM medicines;

-- Sale lines record the selling unit + the base-unit quantity consumed. Existing
-- rows were already in the base unit, so unit_factor=1 and base_qty=qty.
ALTER TABLE sale_items
  ADD COLUMN medicine_unit_id UUID,
  ADD COLUMN unit_name TEXT NOT NULL DEFAULT '',
  ADD COLUMN unit_factor BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN base_qty INT NOT NULL DEFAULT 0;

UPDATE sale_items SET base_qty = qty WHERE base_qty = 0;
UPDATE sale_items si SET medicine_unit_id = mu.id, unit_name = mu.name
FROM medicine_units mu
WHERE mu.medicine_id = si.medicine_id AND mu.is_base AND si.medicine_unit_id IS NULL;

-- +goose Down
ALTER TABLE sale_items
  DROP COLUMN IF EXISTS medicine_unit_id,
  DROP COLUMN IF EXISTS unit_name,
  DROP COLUMN IF EXISTS unit_factor,
  DROP COLUMN IF EXISTS base_qty;
DROP TABLE IF EXISTS medicine_units;
