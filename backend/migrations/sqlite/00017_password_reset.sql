-- +goose Up
CREATE TABLE password_reset_tokens (
  id         TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  issued_by  TEXT NOT NULL REFERENCES users(id),
  expires_at DATETIME NOT NULL,
  used_at    DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX password_reset_user_idx     ON password_reset_tokens(user_id);
CREATE INDEX password_reset_unused_idx   ON password_reset_tokens(expires_at) WHERE used_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS password_reset_tokens;
