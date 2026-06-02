-- +goose Up
-- Global unit catalog: owner registers base units (tablet, ml) and their
-- derivatives (strip ×10, box ×100, liter ×1000). Managed in Settings.
-- Existing per-medicine `medicine_units` rows are unchanged; the catalog is
-- a UX preset library that seeds the medicine form on create/edit.
CREATE TABLE unit_bases (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL UNIQUE,
  active     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE unit_derivatives (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  base_unit_id UUID NOT NULL REFERENCES unit_bases(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  factor       BIGINT NOT NULL CHECK (factor > 1),
  sort_order   INTEGER NOT NULL DEFAULT 0,
  active       BOOLEAN NOT NULL DEFAULT TRUE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX unit_derivatives_unique_active
  ON unit_derivatives (base_unit_id, name) WHERE active;

-- +goose Down
DROP TABLE IF EXISTS unit_derivatives;
DROP TABLE IF EXISTS unit_bases;
