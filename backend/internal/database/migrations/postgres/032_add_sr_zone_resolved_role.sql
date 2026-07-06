-- +goose Up
ALTER TABLE stock_sr_zones ADD COLUMN resolved_role VARCHAR(15) NULL;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN resolved_role;
