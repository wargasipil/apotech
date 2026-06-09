-- +goose Up
-- SQLite carve-out: the portable build is fresh-DB only, so the LPAD/window
-- backfill from the Postgres set isn't needed. We just add the column NOT
-- NULL with an empty default; CreateSupplier always supplies a unique code.
ALTER TABLE suppliers ADD COLUMN code TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX suppliers_code_idx ON suppliers(code);

-- +goose Down
DROP INDEX IF EXISTS suppliers_code_idx;
