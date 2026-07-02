-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_zones (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    analysis_id             BIGINT UNSIGNED NOT NULL,
    price_low               DECIMAL(10,2) NOT NULL,
    price_high              DECIMAL(10,2) NOT NULL,
    method                  VARCHAR(20)   NOT NULL,
    role                    VARCHAR(15)   NOT NULL,
    support_score           DECIMAL(6,4)  NOT NULL,
    resistance_score        DECIMAL(6,4)  NOT NULL,
    bounce_probability      DECIMAL(6,4),
    break_probability       DECIMAL(6,4),
    touch_count             INT           NOT NULL,
    rejection_count         INT           NOT NULL,
    breakout_count          INT           NOT NULL,
    avg_return_after_touch  DECIMAL(10,6) NOT NULL,
    relative_volume         DECIMAL(10,6) NOT NULL,
    volatility              DECIMAL(10,6) NOT NULL,
    trend_strength          DECIMAL(10,6) NOT NULL,
    status                  VARCHAR(15)   NOT NULL DEFAULT 'PENDING',
    broken_at               DATETIME(0),
    broken_price            DECIMAL(10,2),
    FOREIGN KEY (analysis_id) REFERENCES stock_sr_zone_analyses(id),
    INDEX idx_analysis_id (analysis_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS stock_sr_zones;
