-- +goose Up
-- Per-year prescription numbering.
CREATE TABLE rx_no_counters (
  year     INT PRIMARY KEY,
  last_seq INT NOT NULL DEFAULT 0
);

CREATE TABLE prescriptions (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  rx_no         TEXT UNIQUE,                       -- assigned on create
  customer_id   UUID NOT NULL REFERENCES customers(id),
  issuer_name   TEXT NOT NULL,                     -- free-text doctor name
  issued_at     DATE NOT NULL,
  expires_at    DATE NOT NULL,
  note          TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','VOIDED')),
  created_by    UUID NOT NULL REFERENCES users(id),
  branch_id     UUID,                              -- placeholder for multi-branch
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX prescriptions_customer_idx ON prescriptions(customer_id);
CREATE INDEX prescriptions_status_idx   ON prescriptions(status);
CREATE INDEX prescriptions_expires_idx  ON prescriptions(expires_at);

CREATE TABLE prescription_items (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  prescription_id      UUID NOT NULL REFERENCES prescriptions(id) ON DELETE CASCADE,
  medicine_id          UUID NOT NULL REFERENCES medicines(id),
  prescribed_qty       INTEGER NOT NULL CHECK (prescribed_qty > 0),
  dispensed_qty        INTEGER NOT NULL DEFAULT 0 CHECK (dispensed_qty >= 0),
  dosage_instructions  TEXT NOT NULL DEFAULT '',
  note                 TEXT NOT NULL DEFAULT '',
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT prescription_items_dispensed_le_prescribed CHECK (dispensed_qty <= prescribed_qty)
);
CREATE INDEX prescription_items_rx_idx       ON prescription_items(prescription_id);
CREATE INDEX prescription_items_medicine_idx ON prescription_items(medicine_id);

-- Sale links to a prescription when Rx-required medicines are dispensed.
ALTER TABLE sales ADD COLUMN prescription_id UUID REFERENCES prescriptions(id);
CREATE INDEX sales_prescription_idx ON sales(prescription_id) WHERE prescription_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS sales_prescription_idx;
ALTER TABLE sales DROP COLUMN IF EXISTS prescription_id;
DROP TABLE IF EXISTS prescription_items;
DROP TABLE IF EXISTS prescriptions;
DROP TABLE IF EXISTS rx_no_counters;
