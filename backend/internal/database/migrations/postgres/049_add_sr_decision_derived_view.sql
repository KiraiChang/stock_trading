-- +goose Up
ALTER TABLE stock_sr_decisions
    ADD COLUMN decision_derived_view_json JSONB NOT NULL DEFAULT 'null'::jsonb;

-- +goose Down
ALTER TABLE stock_sr_decisions
    DROP COLUMN decision_derived_view_json;
