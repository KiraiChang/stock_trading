-- +goose Up
ALTER TABLE stock_sr_decisions ADD COLUMN decision_derived_view_json TEXT NOT NULL DEFAULT 'null';

-- +goose Down
ALTER TABLE stock_sr_decisions DROP COLUMN decision_derived_view_json;
