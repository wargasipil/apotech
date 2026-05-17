-- +goose Up
-- NSFP (Nomor Seri Faktur Pajak) pool. The PKP-registered seller pre-purchases
-- a range of tax invoice numbers from DJP via e-Nofa; we record each available
-- number here and consume the lowest-unused one when a tax invoice is assigned
-- to a sale.
CREATE TABLE nsfp_pool (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code          TEXT NOT NULL UNIQUE,        -- full 13-char NSFP body, e.g. "000.24.12345678"
  fiscal_year   INT  NOT NULL,
  imported_by   UUID NOT NULL REFERENCES users(id),
  imported_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  used_at       TIMESTAMPTZ,
  sale_id       UUID REFERENCES sales(id),
  CONSTRAINT nsfp_used_has_sale CHECK ((used_at IS NULL) = (sale_id IS NULL))
);
CREATE INDEX nsfp_unused_idx ON nsfp_pool(fiscal_year, code) WHERE used_at IS NULL;
CREATE INDEX nsfp_sale_idx   ON nsfp_pool(sale_id) WHERE sale_id IS NOT NULL;

-- Customers can have an NPWP (15-digit tax ID, formatted "XX.XXX.XXX.X-XXX.XXX").
-- We treat is_pkp as derived (NPWP set => the customer is PKP/B2B).
ALTER TABLE customers ADD COLUMN npwp    TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN address TEXT NOT NULL DEFAULT '';

-- Tax-invoice metadata on sales. The `tax_invoice_no` slot already exists from
-- Phase 3; we add the issuance timestamp and the transaction code (default "01"
-- = standard PKP sale). DPP/PPN amounts are computed from `total` at assignment
-- time (PPN 11% as of 2026).
ALTER TABLE sales ADD COLUMN tax_invoice_code   TEXT;             -- "01"|"02"|...
ALTER TABLE sales ADD COLUMN tax_invoice_dpp    BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sales ADD COLUMN tax_invoice_ppn    BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sales ADD COLUMN tax_invoice_issued_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE sales DROP COLUMN IF EXISTS tax_invoice_issued_at;
ALTER TABLE sales DROP COLUMN IF EXISTS tax_invoice_ppn;
ALTER TABLE sales DROP COLUMN IF EXISTS tax_invoice_dpp;
ALTER TABLE sales DROP COLUMN IF EXISTS tax_invoice_code;
ALTER TABLE customers DROP COLUMN IF EXISTS address;
ALTER TABLE customers DROP COLUMN IF EXISTS npwp;
DROP TABLE IF EXISTS nsfp_pool;
