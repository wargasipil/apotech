-- +goose Up
-- Removed: supplier (set on every batch at receive time) already covers the
-- "where this lot came from" question; manufacturer was unused.
ALTER TABLE medicines DROP COLUMN manufacturer;

-- +goose Down
ALTER TABLE medicines
  ADD COLUMN manufacturer TEXT NOT NULL DEFAULT '';
