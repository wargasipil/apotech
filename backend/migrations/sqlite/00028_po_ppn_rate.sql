-- +goose Up
ALTER TABLE purchase_orders ADD COLUMN ppn_rate INTEGER NOT NULL DEFAULT 11;

-- +goose Down
SELECT 1;
