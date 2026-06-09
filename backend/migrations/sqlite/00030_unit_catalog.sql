-- +goose Up
CREATE TABLE unit_bases (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL UNIQUE,
  active     BOOLEAN NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE unit_derivatives (
  id           TEXT PRIMARY KEY,
  base_unit_id TEXT NOT NULL REFERENCES unit_bases(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  factor       INTEGER NOT NULL CHECK (factor > 1),
  sort_order   INTEGER NOT NULL DEFAULT 0,
  active       BOOLEAN NOT NULL DEFAULT 1,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX unit_derivatives_unique_active
  ON unit_derivatives (base_unit_id, name) WHERE active = 1;

-- +goose Down
DROP TABLE IF EXISTS unit_derivatives;
DROP TABLE IF EXISTS unit_bases;
