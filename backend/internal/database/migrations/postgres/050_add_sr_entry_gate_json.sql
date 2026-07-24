-- +goose Up
ALTER TABLE stock_sr_decisions
    ADD COLUMN entry_executability_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN entry_blocking_zone_json JSONB NOT NULL DEFAULT 'null'::jsonb;

-- +goose Down
ALTER TABLE stock_sr_decisions
    DROP COLUMN entry_blocking_zone_json,
    DROP COLUMN entry_executability_json;
