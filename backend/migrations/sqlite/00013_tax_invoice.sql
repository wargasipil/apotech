-- +goose Up
CREATE TABLE nsfp_pool (
  id            TEXT PRIMARY KEY,
  code          TEXT NOT NULL UNIQUE,
  fiscal_year   INTEGER NOT NULL,
  imported_by   TEXT NOT NULL REFERENCES users(id),
  imported_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  used_at       DATETIME,
  sale_id       TEXT REFERENCES sales(id),
  CONSTRAINT nsfp_used_has_sale CHECK ((used_at IS NULL) = (sale_id IS NULL))
);
CREATE INDEX nsfp_unused_idx ON nsfp_pool(fiscal_year, code) WHERE used_at IS NULL;
CREATE INDEX nsfp_sale_idx   ON nsfp_pool(sale_id) WHERE sale_id IS NOT NULL;

ALTER TABLE customers ADD COLUMN npwp    TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN address TEXT NOT NULL DEFAULT '';

ALTER TABLE sales ADD COLUMN tax_invoice_code   TEXT;
ALTER TABLE sales ADD COLUMN tax_invoice_dpp    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sales ADD COLUMN tax_invoice_ppn    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sales ADD COLUMN tax_invoice_issued_at DATETIME;

-- +goose Down
DROP TABLE IF EXISTS nsfp_pool;
