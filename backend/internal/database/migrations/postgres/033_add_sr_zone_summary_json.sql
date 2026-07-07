-- +goose Up
ALTER TABLE stock_sr_zone_analyses ADD COLUMN period_summaries TEXT NOT NULL DEFAULT '[]';
ALTER TABLE stock_sr_zone_analyses ADD COLUMN analysis_tips TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE stock_sr_zone_analyses DROP COLUMN analysis_tips;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN period_summaries;
