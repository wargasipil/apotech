-- +goose Up
CREATE TABLE suppliers (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  contact_email TEXT COLLATE NOCASE,
  phone         TEXT,
  active        BOOLEAN NOT NULL DEFAULT 1,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX suppliers_name_active_idx ON suppliers(name) WHERE active = 1;

-- +goose Down
DROP TABLE suppliers;
