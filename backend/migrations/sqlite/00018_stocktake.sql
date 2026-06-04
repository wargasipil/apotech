-- +goose Up
-- Stocktake (stock opname) sessions. A session groups a single physical
-- count event and its resulting adjustments / write-offs.
CREATE TABLE stocktake_sessions (
  id           TEXT PRIMARY KEY,
  name         TEXT NOT NULL DEFAULT '',                 -- human-readable label
  status       TEXT NOT NULL DEFAULT 'DRAFT'
    CHECK (status IN ('DRAFT','COMPLETED','VOIDED')),
  branch_id    TEXT,                                     -- placeholder for multi-branch phase
  created_by   TEXT NOT NULL REFERENCES users(id),
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  completed_at DATETIME,
  voided_at    DATETIME
);
CREATE INDEX stocktake_sessions_status_idx  ON stocktake_sessions(status);
CREATE INDEX stocktake_sessions_created_idx ON stocktake_sessions(created_at DESC);

-- One row per batch the pharmacist intends to count in this session.
-- `expected_qty` is snapshotted at line creation; `counted_qty` is filled in
-- when the pharmacist walks the shelves. Variance = counted - expected.
-- When variance != 0 and the line has been counted, CompleteStocktake writes
-- a stock_movements row whose type comes from `disposition`.
CREATE TABLE stocktake_lines (
  id                TEXT PRIMARY KEY,
  session_id        TEXT NOT NULL REFERENCES stocktake_sessions(id) ON DELETE CASCADE,
  batch_id          TEXT NOT NULL REFERENCES batches(id),
  expected_qty      INTEGER NOT NULL,
  counted_qty       INTEGER,
  disposition       TEXT NOT NULL DEFAULT 'ADJUSTMENT'
    CHECK (disposition IN ('ADJUSTMENT','WRITE_OFF')),
  write_off_kind    TEXT
    CHECK (write_off_kind IS NULL OR write_off_kind IN ('EXPIRED','DAMAGED','LOST','THEFT','OTHER')),
  disposition_note  TEXT NOT NULL DEFAULT '',
  counted_at        DATETIME,
  counted_by        TEXT REFERENCES users(id),
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (session_id, batch_id)
);
CREATE INDEX stocktake_lines_session_idx ON stocktake_lines(session_id);
CREATE INDEX stocktake_lines_batch_idx   ON stocktake_lines(batch_id);

-- Audit chain on the stock_movements ledger: which stocktake line caused
-- this movement (NULL = ad-hoc Record Movement, not from a stocktake), and
-- (for stocktake-driven WRITE_OFF movements) what kind of loss it was.
ALTER TABLE stock_movements ADD COLUMN stocktake_line_id TEXT REFERENCES stocktake_lines(id);
ALTER TABLE stock_movements ADD COLUMN write_off_kind TEXT
  CHECK (write_off_kind IS NULL OR write_off_kind IN ('EXPIRED','DAMAGED','LOST','THEFT','OTHER'));
CREATE INDEX stock_movements_stocktake_idx ON stock_movements(stocktake_line_id)
  WHERE stocktake_line_id IS NOT NULL;
CREATE INDEX stock_movements_writeoff_idx  ON stock_movements(write_off_kind)
  WHERE write_off_kind IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS stock_movements_writeoff_idx;
DROP INDEX IF EXISTS stock_movements_stocktake_idx;
ALTER TABLE stock_movements DROP COLUMN write_off_kind;
ALTER TABLE stock_movements DROP COLUMN stocktake_line_id;
DROP TABLE IF EXISTS stocktake_lines;
DROP TABLE IF EXISTS stocktake_sessions;
