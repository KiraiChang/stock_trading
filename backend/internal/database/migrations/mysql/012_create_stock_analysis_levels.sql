-- +goose Up
CREATE TABLE IF NOT EXISTS stock_analysis_levels (
    id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    analysis_id  BIGINT UNSIGNED NOT NULL,
    price        DECIMAL(10,2)   NOT NULL,
    type         VARCHAR(10)     NOT NULL,
    strength     DECIMAL(6,4)    NOT NULL,
    method       VARCHAR(30)     NOT NULL,
    status       VARCHAR(15)     NOT NULL DEFAULT 'PENDING',
    broken_at    DATETIME(0),
    broken_price DECIMAL(10,2),
    FOREIGN KEY (analysis_id) REFERENCES stock_analyses(id),
    INDEX idx_analysis_id (analysis_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS stock_analysis_levels;
