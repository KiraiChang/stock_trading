-- +goose Up
-- 臨時休市與推翻日曆的手動設定（I-139）。欄位說明見 postgres 版與 docs/database-schema.md。
CREATE TABLE IF NOT EXISTS market_day_overrides (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    trade_date  VARCHAR(10)  NOT NULL,
    action      VARCHAR(10)  NOT NULL,
    reason      VARCHAR(200) NOT NULL,
    user_id     BIGINT UNSIGNED NOT NULL,
    created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_market_day_overrides_date (trade_date, id),
    CONSTRAINT ck_market_day_overrides_action CHECK (action IN ('closed', 'open', 'cleared')),
    CONSTRAINT ck_market_day_overrides_date CHECK (trade_date REGEXP '^[0-9]{4}-[0-9]{2}-[0-9]{2}$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS market_day_overrides;
