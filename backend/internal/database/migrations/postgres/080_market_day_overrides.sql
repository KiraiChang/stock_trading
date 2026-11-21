-- +goose Up
-- 臨時休市（颱風假）與推翻日曆的手動設定（I-139）。**只新增、⛔ 不改寫**：每次儲存與取消各一列，
-- 某一天目前的狀態＝該日 id 最大的那一列（cleared 或沒有列＝依日曆）。現況規格見 docs/database-schema.md。
-- trade_date 是台北的日曆日期字串 YYYY-MM-DD（⛔ 不用 DATE：三種 DB 與 driver 的時區處理不同）。
CREATE TABLE IF NOT EXISTS market_day_overrides (
    id          BIGSERIAL    PRIMARY KEY,
    trade_date  VARCHAR(10)  NOT NULL,
    action      VARCHAR(10)  NOT NULL,
    reason      VARCHAR(200) NOT NULL,
    user_id     BIGINT       NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_market_day_overrides_action CHECK (action IN ('closed', 'open', 'cleared')),
    CONSTRAINT ck_market_day_overrides_date CHECK (trade_date ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$')
);
CREATE INDEX IF NOT EXISTS idx_market_day_overrides_date ON market_day_overrides (trade_date, id);

-- +goose Down
DROP TABLE IF EXISTS market_day_overrides;
