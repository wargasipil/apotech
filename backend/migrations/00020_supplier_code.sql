-- +goose Up
-- Suppliers (pemasok) get a unique business code, like medicines (SKU) and
-- warehouses. Editable + globally unique. Backfill existing rows with SUP-NNNN
-- (the owner can rename them later) before locking NOT NULL + unique.
ALTER TABLE suppliers ADD COLUMN code TEXT;

UPDATE suppliers s
SET code = 'SUP-' || LPAD(seq::text, 4, '0')
FROM (
  SELECT id, row_number() OVER (ORDER BY created_at, id) AS seq
  FROM suppliers
) AS numbered
WHERE s.id = numbered.id;

ALTER TABLE suppliers ALTER COLUMN code SET NOT NULL;
CREATE UNIQUE INDEX suppliers_code_idx ON suppliers(code);

-- +goose Down
DROP INDEX IF EXISTS suppliers_code_idx;
ALTER TABLE suppliers DROP COLUMN IF EXISTS code;
