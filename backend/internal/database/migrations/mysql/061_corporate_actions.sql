-- 公司行動（分割／除權息）與 K 棒的還原係數。背景見 docs/database-schema.md 的「股價還原」。
--
-- 設計：candles 的原始價**不改動**，另存累積還原係數，讀取端相乘。
-- adj_factor 是 corporate_actions 的純函數（見 market/adjuster.go），重算永遠整段覆寫，
-- 所以跑幾次結果都一致。

-- +goose Up
CREATE TABLE IF NOT EXISTS corporate_actions (
    id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol       VARCHAR(10)    NOT NULL,
    event_date   DATE           NOT NULL,
    action_type  VARCHAR(16)    NOT NULL,
    before_price DECIMAL(10,2)  NOT NULL,
    after_price  DECIMAL(10,2)  NOT NULL,
    factor       DECIMAL(18,10) NOT NULL,
    source       VARCHAR(32)    NOT NULL,
    created_at   DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- 冪等性的第一道保證：重複抓取不會產生第二筆事件，否則同一次分割會被乘兩次。
    UNIQUE KEY uk_symbol_date_type (symbol, event_date, action_type),
    INDEX idx_symbol_date (symbol, event_date),
    CONSTRAINT ck_corporate_actions_prices CHECK (before_price > 0 AND after_price > 0 AND factor > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 預設 1 ＝「未調整」，語意明確且沒有中間狀態：重算還沒跑過的資料就是原始價。
ALTER TABLE candles ADD COLUMN adj_factor DECIMAL(18,10) NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE candles DROP COLUMN adj_factor;
DROP TABLE IF EXISTS corporate_actions;
