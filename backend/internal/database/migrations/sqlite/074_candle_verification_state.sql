-- 日 K 缺漏偵測的逐標的驗證簿記（現況見 docs/database-schema.md 與 docs/architecture.md；
-- 原記於 issue.md I-091，已收斂）。
-- 完整設計理由見 postgres 版的註解與 docs/database-schema.md。
--
-- sqlite 差異：TEXT 代替 VARCHAR、時間用 DATETIME。
-- CHECK constraint sqlite 原生支援且會強制執行。

-- +goose Up
CREATE TABLE IF NOT EXISTS candle_verification_state (
    symbol    TEXT NOT NULL,
    timeframe TEXT NOT NULL,

    last_attempted_at DATETIME NULL,
    last_verified_at  DATETIME NULL,

    -- verified / gap / deferred / unavailable，**無 DEFAULT，寫入時必填**。
    last_result TEXT NOT NULL,

    consecutive_failures INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY (symbol, timeframe),
    CONSTRAINT ck_candle_verification_last_result
        CHECK (last_result IN ('verified', 'gap', 'deferred', 'unavailable'))
);

-- +goose Down
DROP TABLE IF EXISTS candle_verification_state;
