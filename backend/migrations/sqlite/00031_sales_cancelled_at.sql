-- +goose Up
ALTER TABLE sales ADD COLUMN cancelled_at DATETIME;

-- +goose Down
ALTER TABLE sales DROP COLUMN cancelled_at;
