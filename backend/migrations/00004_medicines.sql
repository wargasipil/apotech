-- +goose Up
CREATE TABLE medicines (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sku                   TEXT UNIQUE NOT NULL,
  name                  TEXT NOT NULL,
  manufacturer          TEXT NOT NULL DEFAULT '',
  unit                  TEXT NOT NULL,
  unit_price            BIGINT NOT NULL,
  prescription_required BOOLEAN NOT NULL DEFAULT FALSE,
  active                BOOLEAN NOT NULL DEFAULT TRUE,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX medicines_name_idx ON medicines(name);

-- +goose Down
DROP TABLE medicines;
