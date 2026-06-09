-- +goose Up
CREATE TABLE refresh_tokens (
  id         TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL REFERENCES users(id),
  token_hash TEXT NOT NULL UNIQUE,
  family_id  TEXT NOT NULL,
  parent_id  TEXT REFERENCES refresh_tokens(id),
  expires_at DATETIME NOT NULL,
  revoked_at DATETIME,
  user_agent TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX refresh_tokens_user_idx    ON refresh_tokens(user_id);
CREATE INDEX refresh_tokens_family_idx  ON refresh_tokens(family_id);
CREATE INDEX refresh_tokens_expires_idx ON refresh_tokens(expires_at);

-- +goose Down
DROP TABLE refresh_tokens;
