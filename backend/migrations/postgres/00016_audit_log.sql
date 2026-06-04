-- +goose Up
CREATE TABLE audit_log (
  id         BIGSERIAL PRIMARY KEY,
  user_id    UUID REFERENCES users(id),
  role       TEXT NOT NULL DEFAULT '',
  branch_id  UUID,
  procedure  TEXT NOT NULL,
  ok         BOOLEAN NOT NULL,
  code       TEXT NOT NULL DEFAULT '',     -- Connect error code, "" on success
  message    TEXT NOT NULL DEFAULT '',     -- error message, "" on success
  ip         TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  duration_ms INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_user_idx      ON audit_log(user_id, created_at DESC);
CREATE INDEX audit_log_procedure_idx ON audit_log(procedure, created_at DESC);
CREATE INDEX audit_log_created_idx   ON audit_log(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
