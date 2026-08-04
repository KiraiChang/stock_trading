-- +goose Up
ALTER TABLE stock_sr_regression_results ADD COLUMN schema_version VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE stock_sr_regression_results ADD COLUMN result_rows INTEGER;
ALTER TABLE stock_sr_regression_results ADD COLUMN source_count INTEGER;
ALTER TABLE stock_sr_regression_results ADD COLUMN governance_health_state VARCHAR(40) NOT NULL DEFAULT '';
ALTER TABLE stock_sr_regression_results ADD COLUMN governance_strict_passed BOOLEAN;
CREATE INDEX IF NOT EXISTS idx_stock_sr_regression_results_schema ON stock_sr_regression_results(schema_version, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stock_sr_regression_results_governance ON stock_sr_regression_results(governance_health_state, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_stock_sr_regression_results_governance;
DROP INDEX IF EXISTS idx_stock_sr_regression_results_schema;
ALTER TABLE stock_sr_regression_results DROP COLUMN governance_strict_passed;
ALTER TABLE stock_sr_regression_results DROP COLUMN governance_health_state;
ALTER TABLE stock_sr_regression_results DROP COLUMN source_count;
ALTER TABLE stock_sr_regression_results DROP COLUMN result_rows;
ALTER TABLE stock_sr_regression_results DROP COLUMN schema_version;
