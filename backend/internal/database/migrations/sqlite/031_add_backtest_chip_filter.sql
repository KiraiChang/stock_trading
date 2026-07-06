-- +goose Up
ALTER TABLE backtest_jobs ADD COLUMN use_chip_filter INTEGER NOT NULL DEFAULT 0;
ALTER TABLE backtest_jobs ADD COLUMN chip_min_score REAL NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE backtest_jobs DROP COLUMN use_chip_filter;
ALTER TABLE backtest_jobs DROP COLUMN chip_min_score;
