-- +goose Up
CREATE TABLE medicines (
  id                    TEXT PRIMARY KEY,
  sku                   TEXT UNIQUE NOT NULL,
  name                  TEXT NOT NULL,
  manufacturer          TEXT NOT NULL DEFAULT '',
  unit                  TEXT NOT NULL,
  unit_price            INTEGER NOT NULL,
  prescription_required BOOLEAN NOT NULL DEFAULT 0,
  active                BOOLEAN NOT NULL DEFAULT 1,
  created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX medicines_name_idx ON medicines(name);

-- +goose Down
DROP TABLE medicines;
