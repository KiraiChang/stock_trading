-- +goose Up
ALTER TABLE stock_sr_regression_results ADD COLUMN IF NOT EXISTS schema_version VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE stock_sr_regression_results ADD COLUMN IF NOT EXISTS result_rows INTEGER;
ALTER TABLE stock_sr_regression_results ADD COLUMN IF NOT EXISTS source_count INTEGER;
ALTER TABLE stock_sr_regression_results ADD COLUMN IF NOT EXISTS governance_health_state VARCHAR(40) NOT NULL DEFAULT '';
ALTER TABLE stock_sr_regression_results ADD COLUMN IF NOT EXISTS governance_strict_passed BOOLEAN;
CREATE INDEX IF NOT EXISTS idx_stock_sr_regression_results_schema ON stock_sr_regression_results(schema_version, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stock_sr_regression_results_governance ON stock_sr_regression_results(governance_health_state, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_stock_sr_regression_results_governance;
DROP INDEX IF EXISTS idx_stock_sr_regression_results_schema;
ALTER TABLE stock_sr_regression_results DROP COLUMN IF EXISTS governance_strict_passed;
ALTER TABLE stock_sr_regression_results DROP COLUMN IF EXISTS governance_health_state;
ALTER TABLE stock_sr_regression_results DROP COLUMN IF EXISTS source_count;
ALTER TABLE stock_sr_regression_results DROP COLUMN IF EXISTS result_rows;
ALTER TABLE stock_sr_regression_results DROP COLUMN IF EXISTS schema_version;
