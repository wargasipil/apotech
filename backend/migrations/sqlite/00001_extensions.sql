-- +goose Up
-- SQLite ships no extension layer; case-insensitive text uses COLLATE NOCASE
-- on individual columns (see the users table for the canonical example).
-- This migration is a no-op so version numbers stay aligned with the
-- Postgres set.
SELECT 1;

-- +goose Down
SELECT 1;
