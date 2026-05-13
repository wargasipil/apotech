-- +goose Up
CREATE TABLE suppliers (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name          TEXT NOT NULL,
  contact_email CITEXT,
  phone         TEXT,
  active        BOOLEAN NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX suppliers_name_active_idx ON suppliers(name) WHERE active = TRUE;

-- +goose Down
DROP TABLE suppliers;
