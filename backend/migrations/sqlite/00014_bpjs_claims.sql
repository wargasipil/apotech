-- +goose Up
CREATE TABLE bpjs_claims (
  id            TEXT PRIMARY KEY,
  sale_id       TEXT NOT NULL REFERENCES sales(id),
  customer_id   TEXT NOT NULL REFERENCES customers(id),
  bpjs_no       TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'DRAFT'
    CHECK (status IN ('DRAFT','SUBMITTED','APPROVED','REJECTED','PAID')),
  amount        INTEGER NOT NULL DEFAULT 0,
  external_ref  TEXT NOT NULL DEFAULT '',
  note          TEXT NOT NULL DEFAULT '',
  submitted_at  DATETIME,
  resolved_at   DATETIME,
  created_by    TEXT NOT NULL REFERENCES users(id),
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX bpjs_claims_sale_idx     ON bpjs_claims(sale_id);
CREATE INDEX bpjs_claims_customer_idx ON bpjs_claims(customer_id);
CREATE INDEX bpjs_claims_status_idx   ON bpjs_claims(status);

-- +goose Down
DROP TABLE IF EXISTS bpjs_claims;
