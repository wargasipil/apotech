-- +goose Up
-- BIGSERIAL has no SQLite equivalent; INTEGER PRIMARY KEY auto-assigns rowid
-- (the model's autoIncrement tag is driver-neutral and the UUID hook skips
-- non-string PKs, so this stays integer-keyed like Postgres).
CREATE TABLE audit_log (
  id         INTEGER PRIMARY KEY,
  user_id    TEXT REFERENCES users(id),
  role       TEXT NOT NULL DEFAULT '',
  branch_id  TEXT,
  procedure  TEXT NOT NULL,
  ok         BOOLEAN NOT NULL,
  code       TEXT NOT NULL DEFAULT '',     -- Connect error code, "" on success
  message    TEXT NOT NULL DEFAULT '',     -- error message, "" on success
  ip         TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  duration_ms INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX audit_log_user_idx      ON audit_log(user_id, created_at DESC);
CREATE INDEX audit_log_procedure_idx ON audit_log(procedure, created_at DESC);
CREATE INDEX audit_log_created_idx   ON audit_log(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
