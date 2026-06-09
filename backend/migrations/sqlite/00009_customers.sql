-- +goose Up
CREATE TABLE customers (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  phone      TEXT NOT NULL DEFAULT '',
  bpjs_no    TEXT NOT NULL DEFAULT '',
  notes      TEXT NOT NULL DEFAULT '',
  active     BOOLEAN NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX customers_name_idx ON customers(name);
CREATE INDEX customers_phone_idx ON customers(phone) WHERE phone <> '';
CREATE INDEX customers_bpjs_idx ON customers(bpjs_no) WHERE bpjs_no <> '';

-- +goose Down
DROP TABLE customers;
