-- +goose Up
ALTER TABLE stock_sr_zones ADD COLUMN confidence REAL NOT NULL DEFAULT 0;
ALTER TABLE stock_sr_zones ADD COLUMN expected_value REAL;
ALTER TABLE stock_sr_zones ADD COLUMN risk_reward_ratio REAL;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN confidence;
ALTER TABLE stock_sr_zones DROP COLUMN expected_value;
ALTER TABLE stock_sr_zones DROP COLUMN risk_reward_ratio;
