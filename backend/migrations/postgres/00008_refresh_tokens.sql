-- +goose Up
CREATE TABLE refresh_tokens (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES users(id),
  token_hash TEXT NOT NULL UNIQUE,
  family_id  UUID NOT NULL,
  parent_id  UUID REFERENCES refresh_tokens(id),
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  user_agent TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx    ON refresh_tokens(user_id);
CREATE INDEX refresh_tokens_family_idx  ON refresh_tokens(family_id);
CREATE INDEX refresh_tokens_expires_idx ON refresh_tokens(expires_at);

-- +goose Down
DROP TABLE refresh_tokens;
