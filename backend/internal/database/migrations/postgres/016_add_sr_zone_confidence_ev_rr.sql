-- +goose Up
ALTER TABLE stock_sr_zones ADD COLUMN confidence DECIMAL(6,4) NOT NULL DEFAULT 0;
ALTER TABLE stock_sr_zones ADD COLUMN expected_value DECIMAL(10,6);
ALTER TABLE stock_sr_zones ADD COLUMN risk_reward_ratio DECIMAL(10,4);

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN confidence;
ALTER TABLE stock_sr_zones DROP COLUMN expected_value;
ALTER TABLE stock_sr_zones DROP COLUMN risk_reward_ratio;
