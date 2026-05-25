-- +goose Up
-- Per-unit sell-price history, mirroring medicine_prices but keyed by the unit.
-- Exactly one open row (effective_to IS NULL) per unit. changed_by is nullable so
-- the backfill (system-seeded baseline) can insert without a user.
CREATE TABLE medicine_unit_prices (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  medicine_unit_id UUID NOT NULL REFERENCES medicine_units(id),
  unit_sell_price  BIGINT NOT NULL,
  effective_from   TIMESTAMPTZ NOT NULL DEFAULT now(),
  effective_to     TIMESTAMPTZ,
  changed_by       UUID REFERENCES users(id),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX medicine_unit_prices_open_idx
  ON medicine_unit_prices(medicine_unit_id)
  WHERE effective_to IS NULL;
CREATE INDEX medicine_unit_prices_unit_idx ON medicine_unit_prices(medicine_unit_id);

-- Backfill: one open row per existing unit at its current sell_price.
INSERT INTO medicine_unit_prices (medicine_unit_id, unit_sell_price, effective_from)
SELECT id, sell_price, created_at FROM medicine_units;

-- +goose Down
DROP TABLE medicine_unit_prices;
