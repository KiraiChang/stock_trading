-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_regression_results (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    run_id VARCHAR(64) NOT NULL UNIQUE,
    model_config_hash VARCHAR(40) NOT NULL DEFAULT '',
    pipeline_version VARCHAR(40) NOT NULL DEFAULT '',
    dataset_from DATETIME,
    dataset_to DATETIME,
    split_method VARCHAR(20) NOT NULL DEFAULT '',
    hold_auc DOUBLE,
    hold_brier_score DOUBLE,
    break_auc DOUBLE,
    break_brier_score DOUBLE,
    passed BOOLEAN,
    metrics_json LONGTEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_stock_sr_regression_results_config (model_config_hash, created_at),
    INDEX idx_stock_sr_regression_results_passed (passed, created_at)
);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_regression_results;
