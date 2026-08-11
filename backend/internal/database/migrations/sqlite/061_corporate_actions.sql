-- 公司行動（分割／除權息）與 K 棒的還原係數。背景見 docs/database-schema.md 的「股價還原」。
--
-- 設計：candles 的原始價**不改動**，另存累積還原係數，讀取端相乘。
-- adj_factor 是 corporate_actions 的純函數（見 market/adjuster.go），重算永遠整段覆寫，
-- 所以跑幾次結果都一致。
--
-- sqlite 支援 ALTER TABLE ADD COLUMN，所以這支不需要重建 candles。

-- +goose Up
CREATE TABLE IF NOT EXISTS corporate_actions (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol       TEXT    NOT NULL,
    event_date   DATE    NOT NULL,
    action_type  TEXT    NOT NULL,
    before_price REAL    NOT NULL,
    after_price  REAL    NOT NULL,
    factor       REAL    NOT NULL,
    source       TEXT    NOT NULL,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- 冪等性的第一道保證：重複抓取不會產生第二筆事件，否則同一次分割會被乘兩次。
    UNIQUE (symbol, event_date, action_type),
    CONSTRAINT ck_corporate_actions_prices CHECK (before_price > 0 AND after_price > 0 AND factor > 0)
);
CREATE INDEX IF NOT EXISTS idx_corporate_actions_symbol_date ON corporate_actions(symbol, event_date);

ALTER TABLE candles ADD COLUMN adj_factor REAL NOT NULL DEFAULT 1;

-- +goose Down
-- sqlite 的 DROP COLUMN 需要 3.35+；modernc.org/sqlite 已支援。
ALTER TABLE candles DROP COLUMN adj_factor;
DROP TABLE IF EXISTS corporate_actions;
