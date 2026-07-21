-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_regression_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id VARCHAR(64) NOT NULL UNIQUE,
    model_config_hash VARCHAR(40) NOT NULL DEFAULT '',
    pipeline_version VARCHAR(40) NOT NULL DEFAULT '',
    dataset_from DATETIME,
    dataset_to DATETIME,
    split_method VARCHAR(20) NOT NULL DEFAULT '',
    hold_auc REAL,
    hold_brier_score REAL,
    break_auc REAL,
    break_brier_score REAL,
    passed BOOLEAN,
    metrics_json TEXT NOT NULL DEFAULT 'null',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_stock_sr_regression_results_config ON stock_sr_regression_results(model_config_hash, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stock_sr_regression_results_passed ON stock_sr_regression_results(passed, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_regression_results;
