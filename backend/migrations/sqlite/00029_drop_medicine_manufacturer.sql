-- +goose Up
-- SQLite supports DROP COLUMN since 3.35; modernc.org/sqlite 1.52+ ships
-- SQLite 3.45+, so this works as-is.
ALTER TABLE medicines DROP COLUMN manufacturer;

-- +goose Down
ALTER TABLE medicines ADD COLUMN manufacturer TEXT NOT NULL DEFAULT '';
