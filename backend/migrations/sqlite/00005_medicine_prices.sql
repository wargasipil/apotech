-- +goose Up
CREATE TABLE medicine_prices (
  id             TEXT PRIMARY KEY,
  medicine_id    TEXT NOT NULL REFERENCES medicines(id),
  unit_price     INTEGER NOT NULL,
  effective_from DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  effective_to   DATETIME,
  changed_by     TEXT NOT NULL REFERENCES users(id),
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX medicine_prices_open_idx
  ON medicine_prices(medicine_id)
  WHERE effective_to IS NULL;
CREATE INDEX medicine_prices_medicine_idx ON medicine_prices(medicine_id);

-- +goose Down
DROP TABLE medicine_prices;
