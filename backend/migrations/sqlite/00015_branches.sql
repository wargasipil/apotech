-- +goose Up
CREATE TABLE branches (
  id         TEXT PRIMARY KEY,
  code       TEXT UNIQUE NOT NULL,            -- short slug, e.g. "MAIN", "DPK01"
  name       TEXT NOT NULL,
  address    TEXT NOT NULL DEFAULT '',
  phone      TEXT NOT NULL DEFAULT '',
  active     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_branches (
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  branch_id  TEXT NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, branch_id)
);
CREATE INDEX user_branches_branch_idx ON user_branches(branch_id);
-- One default branch per user.
CREATE UNIQUE INDEX user_branches_default_idx ON user_branches(user_id) WHERE is_default;

-- Seed the legacy single-shop branch and grant access to every existing user.
-- TEXT primary keys carry no DB-side default (the app generates UUIDs Go-side),
-- so this raw seed provides an explicit id.
INSERT INTO branches (id, code, name) VALUES ('00000000-0000-4000-8000-0000000000b1', 'MAIN', 'Main pharmacy');
INSERT INTO user_branches (user_id, branch_id, is_default)
SELECT u.id, b.id, TRUE
FROM users u, branches b
WHERE b.code = 'MAIN';

-- Backfill the placeholder branch_id columns on operational tables. They were
-- declared nullable for exactly this moment; we keep them nullable so legacy
-- rows that pre-date this migration stay legible.
UPDATE sales            SET branch_id = (SELECT id FROM branches WHERE code = 'MAIN') WHERE branch_id IS NULL;
UPDATE sale_items       SET branch_id = (SELECT id FROM branches WHERE code = 'MAIN') WHERE branch_id IS NULL;
UPDATE stock_movements  SET branch_id = (SELECT id FROM branches WHERE code = 'MAIN') WHERE branch_id IS NULL;
UPDATE purchase_orders  SET branch_id = (SELECT id FROM branches WHERE code = 'MAIN') WHERE branch_id IS NULL;
UPDATE prescriptions    SET branch_id = (SELECT id FROM branches WHERE code = 'MAIN') WHERE branch_id IS NULL;

-- +goose Down
-- Don't unwind the backfill (it's data); just drop the new tables.
DROP TABLE IF EXISTS user_branches;
DROP TABLE IF EXISTS branches;
