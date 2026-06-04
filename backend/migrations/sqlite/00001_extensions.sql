-- +goose Up
-- Postgres uses the citext extension for case-insensitive email columns. SQLite
-- has no extension system; the same behavior comes from TEXT COLLATE NOCASE on
-- those columns (declared in the per-table migrations). Nothing to do here.
SELECT 1;

-- +goose Down
SELECT 1;
