-- +goose Up
ALTER TABLE backtest_jobs ADD COLUMN use_chip_filter BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE backtest_jobs ADD COLUMN chip_min_score NUMERIC(6,2) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE backtest_jobs DROP COLUMN use_chip_filter;
ALTER TABLE backtest_jobs DROP COLUMN chip_min_score;
