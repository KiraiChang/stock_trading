-- +goose Up
ALTER TABLE stock_sr_zones ADD COLUMN resolved_role TEXT;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN resolved_role;
