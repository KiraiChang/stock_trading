-- +goose Up
-- 臨時休市與推翻日曆的手動設定（I-139）。欄位說明見 postgres 版與 docs/database-schema.md。
CREATE TABLE IF NOT EXISTS market_day_overrides (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    trade_date  TEXT     NOT NULL,
    action      TEXT     NOT NULL,
    reason      TEXT     NOT NULL,
    user_id     INTEGER  NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_market_day_overrides_action CHECK (action IN ('closed', 'open', 'cleared')),
    CONSTRAINT ck_market_day_overrides_date CHECK (trade_date GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]')
);
CREATE INDEX IF NOT EXISTS idx_market_day_overrides_date ON market_day_overrides (trade_date, id);

-- +goose Down
DROP INDEX IF EXISTS idx_market_day_overrides_date;
DROP TABLE IF EXISTS market_day_overrides;
