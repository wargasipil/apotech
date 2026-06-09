-- +goose Up
-- Record when a COMPLETED sale was cancelled (new VoidSale flow for completed
-- sales). NULL for sales that were voided in DRAFT or never cancelled.
ALTER TABLE sales ADD COLUMN cancelled_at TIMESTAMPTZ NULL;

-- +goose Down
ALTER TABLE sales DROP COLUMN IF EXISTS cancelled_at;
