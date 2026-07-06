-- +goose Up
-- 【籌碼分析整合】回測任務可選擇是否套用籌碼分數 filter（見
-- docs/chip-analysis-design.md 第9節），Python 端逐 bar 比對 chip_scores.total_score
-- 是否達到 chip_min_score 門檻，未達門檻的進場訊號會被濾掉。
ALTER TABLE backtest_jobs ADD COLUMN use_chip_filter TINYINT(1) NOT NULL DEFAULT 0;
ALTER TABLE backtest_jobs ADD COLUMN chip_min_score DECIMAL(6,2) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE backtest_jobs DROP COLUMN use_chip_filter;
ALTER TABLE backtest_jobs DROP COLUMN chip_min_score;
