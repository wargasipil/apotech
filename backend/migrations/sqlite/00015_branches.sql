-- +goose Up
CREATE TABLE branches (
  id         TEXT PRIMARY KEY,
  code       TEXT UNIQUE NOT NULL,
  name       TEXT NOT NULL,
  address    TEXT NOT NULL DEFAULT '',
  phone      TEXT NOT NULL DEFAULT '',
  active     BOOLEAN NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_branches (
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  branch_id  TEXT NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  is_default BOOLEAN NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, branch_id)
);
CREATE INDEX user_branches_branch_idx ON user_branches(branch_id);
CREATE UNIQUE INDEX user_branches_default_idx ON user_branches(user_id) WHERE is_default = 1;

-- Seed the legacy single-shop branch. The id is a fixed UUID so the row is
-- always findable by code; user_branches grants happen on first user create
-- (no users exist yet on a fresh DB).
INSERT INTO branches (id, code, name)
  VALUES ('00000000-0000-0000-0000-00000000b001', 'MAIN', 'Main pharmacy');

-- +goose Down
DROP TABLE IF EXISTS user_branches;
DROP TABLE IF EXISTS branches;
