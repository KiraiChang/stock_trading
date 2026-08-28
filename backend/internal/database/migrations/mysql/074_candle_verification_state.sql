-- 日 K 缺漏偵測的逐標的驗證簿記（現況見 docs/database-schema.md 與 docs/architecture.md；
-- 原記於 issue.md I-091，已收斂）。
-- 完整設計理由見 postgres 版的註解與 docs/database-schema.md。
--
-- mysql 差異：DATETIME(0) 代替 TIMESTAMPTZ（本 schema 一律存 UTC，由應用層負責時區）。
-- **CHECK constraint 自 MySQL 8.0.16 起才會被強制執行**，更早的版本只會解析後忽略——
-- 值域的守門不能只靠它，寫入端本來就要保證。
--
-- **本 engine 從未部署過**（見 docs/issue.md I-054），只由
-- scripts/test-mysql-migrations.sh 驗證 DDL，不涵蓋 repo 層 CRUD。

-- +goose Up
CREATE TABLE IF NOT EXISTS candle_verification_state (
    symbol    VARCHAR(10) NOT NULL,
    timeframe VARCHAR(5)  NOT NULL,

    last_attempted_at DATETIME(0) NULL,
    last_verified_at  DATETIME(0) NULL,

    -- verified / gap / deferred / unavailable，**無 DEFAULT，寫入時必填**。
    last_result VARCHAR(12) NOT NULL,

    consecutive_failures INT NOT NULL DEFAULT 0,

    PRIMARY KEY (symbol, timeframe),
    CONSTRAINT ck_candle_verification_last_result
        CHECK (last_result IN ('verified', 'gap', 'deferred', 'unavailable'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS candle_verification_state;
