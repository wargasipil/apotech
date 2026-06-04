-- +goose Up
-- BPJS Kesehatan claim tracking. Today the integration with the real DJP/BPJS
-- web service is a stub — we record claims locally with a status enum so the
-- UI works and a future round can swap in a real HTTP client without schema
-- changes.
CREATE TABLE bpjs_claims (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id       UUID NOT NULL REFERENCES sales(id),
  customer_id   UUID NOT NULL REFERENCES customers(id),
  bpjs_no       TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'DRAFT'
    CHECK (status IN ('DRAFT','SUBMITTED','APPROVED','REJECTED','PAID')),
  amount        BIGINT NOT NULL DEFAULT 0,
  external_ref  TEXT NOT NULL DEFAULT '',   -- response code from BPJS (when wired)
  note          TEXT NOT NULL DEFAULT '',
  submitted_at  TIMESTAMPTZ,
  resolved_at   TIMESTAMPTZ,
  created_by    UUID NOT NULL REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX bpjs_claims_sale_idx     ON bpjs_claims(sale_id);
CREATE INDEX bpjs_claims_customer_idx ON bpjs_claims(customer_id);
CREATE INDEX bpjs_claims_status_idx   ON bpjs_claims(status);

-- +goose Down
DROP TABLE IF EXISTS bpjs_claims;
