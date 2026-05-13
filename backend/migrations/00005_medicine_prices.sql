-- +goose Up
CREATE TABLE medicine_prices (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  medicine_id    UUID NOT NULL REFERENCES medicines(id),
  unit_price     BIGINT NOT NULL,
  effective_from TIMESTAMPTZ NOT NULL DEFAULT now(),
  effective_to   TIMESTAMPTZ,
  changed_by     UUID NOT NULL REFERENCES users(id),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX medicine_prices_open_idx
  ON medicine_prices(medicine_id)
  WHERE effective_to IS NULL;
CREATE INDEX medicine_prices_medicine_idx ON medicine_prices(medicine_id);

-- +goose Down
DROP TABLE medicine_prices;
