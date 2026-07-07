-- +goose Up
ALTER TABLE stock_sr_zone_analyses ADD COLUMN period_summaries TEXT NULL;
ALTER TABLE stock_sr_zone_analyses ADD COLUMN analysis_tips TEXT NULL;
UPDATE stock_sr_zone_analyses SET period_summaries = '[]' WHERE period_summaries IS NULL;
UPDATE stock_sr_zone_analyses SET analysis_tips = '[]' WHERE analysis_tips IS NULL;

-- +goose Down
ALTER TABLE stock_sr_zone_analyses DROP COLUMN analysis_tips;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN period_summaries;
