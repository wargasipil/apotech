-- +goose Up
-- Suppliers (pemasok) get a unique business code, like medicines (SKU) and
-- warehouses. Editable + globally unique. Backfill existing rows with SUP-NNNN
-- before locking unique. SQLite cannot ALTER a column to NOT NULL, so `code` is
-- added nullable (vs Postgres which sets NOT NULL after backfill); CreateSupplier
-- / UpdateSupplier always set it, so the invariant holds in application code.
ALTER TABLE suppliers ADD COLUMN code TEXT;

-- Backfill (no-op on a fresh DB). Postgres uses UPDATE ... FROM with a
-- row_number() window; SQLite rewrites it as a correlated subquery that counts
-- earlier rows, and printf('%04d', n) replaces LPAD(n::text, 4, '0').
UPDATE suppliers
SET code = 'SUP-' || printf('%04d',
  (SELECT COUNT(*) FROM suppliers s2
   WHERE s2.created_at < suppliers.created_at
      OR (s2.created_at = suppliers.created_at AND s2.id <= suppliers.id))
)
WHERE code IS NULL;

CREATE UNIQUE INDEX suppliers_code_idx ON suppliers(code);

-- +goose Down
DROP INDEX IF EXISTS suppliers_code_idx;
ALTER TABLE suppliers DROP COLUMN code;
