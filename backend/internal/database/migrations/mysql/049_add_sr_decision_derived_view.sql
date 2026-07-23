-- +goose Up
ALTER TABLE stock_sr_decisions ADD COLUMN decision_derived_view_json LONGTEXT NULL;
UPDATE stock_sr_decisions SET decision_derived_view_json = 'null' WHERE decision_derived_view_json IS NULL;
ALTER TABLE stock_sr_decisions MODIFY decision_derived_view_json LONGTEXT NOT NULL;

-- +goose Down
ALTER TABLE stock_sr_decisions DROP COLUMN decision_derived_view_json;
