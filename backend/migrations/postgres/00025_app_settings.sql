-- +goose Up
-- Generic key/value table for app-wide settings (OWNER-editable). Today the
-- only key is `low_stock_threshold` (string-encoded int32, default 10 in
-- service code when the row is absent); the table is open for future keys
-- (receipt header, etc.) without a new migration.
CREATE TABLE app_settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE app_settings;
