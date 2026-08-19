-- 事件身分與生命週期（T-048 階段 C）。完整設計理由見 postgres 版的註解與
-- docs/sr-zone-scoring.md「Zone 身分與 ZoneMatcher」。
--
-- mysql 從未在任何環境部署，唯一的驗證路徑是 scripts/test-mysql-migrations.sh（見 issue.md I-054）。
--
-- mysql 差異：
--   * TIMESTAMPTZ → DATETIME(0)
--   * BIGSERIAL → BIGINT UNSIGNED AUTO_INCREMENT
--   * DROP INDEX 沒有 IF EXISTS，但 Down 直接 DROP TABLE 就會一併移除索引
--   * reason_codes 用 TEXT DEFAULT ('[]') 的括號寫法
--
-- 欄位名都避開了 MySQL 保留字（見 database-schema.md「欄位命名規範」）。

-- +goose Up

CREATE TABLE IF NOT EXISTS event_instances (
    event_uid      VARCHAR(64) NOT NULL PRIMARY KEY,
    symbol         VARCHAR(10) NOT NULL,
    timeframe      VARCHAR(8)  NOT NULL,
    -- 可為 NULL：SYMBOL scope 的事件不屬於任何 zone，見 postgres 版註解
    zone_uid       VARCHAR(64) NULL,
    -- zone_uid 的 NOT NULL 投影，ZONE scope 時等於 zone_uid、SYMBOL scope 時是 'SYMBOL'
    zone_scope_key VARCHAR(64) NOT NULL,
    event_scope    VARCHAR(20) NOT NULL,
    event_family   VARCHAR(80) NOT NULL,
    -- 同一個 (zone, family) 的第幾條鏈，比照 zone_role_incarnations.seq
    seq            INT         NOT NULL DEFAULT 1,
    root_event_type   VARCHAR(80) NOT NULL,
    latest_event_type VARCHAR(80) NOT NULL,
    state          VARCHAR(40) NOT NULL,
    active         BOOLEAN     NOT NULL DEFAULT FALSE,
    direction      VARCHAR(20) NOT NULL,
    resolved_by    VARCHAR(80) NULL,
    first_seen_at  DATETIME(0) NOT NULL,
    last_seen_at   DATETIME(0) NOT NULL,
    ended_at       DATETIME(0) NULL,
    -- 這條鏈最近一次被觀測到時事件身上帶的 zone_key。鏈延續的第一把鑰匙，
    -- 見 postgres 版註解
    last_zone_key  VARCHAR(96) NULL,
    -- RESOLVED / EXPIRED / ZONE_IDENTITY_ENDED
    end_reason     VARCHAR(32) NULL,
    created_at     DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_event_instance_seq UNIQUE (symbol, timeframe, zone_scope_key, event_family, seq),
    CONSTRAINT fk_event_instances_zone FOREIGN KEY (zone_uid) REFERENCES zone_instances(zone_uid),
    INDEX idx_event_instances_live (symbol, timeframe, ended_at, last_seen_at),
    INDEX idx_event_instances_zone (zone_uid),
    -- 第一把鑰匙的查找路徑：(last_zone_key, event_family) → 活鏈。
    INDEX idx_event_instances_last_zone_key (symbol, timeframe, last_zone_key, event_family)
);

CREATE TABLE IF NOT EXISTS event_transitions (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    event_uid   VARCHAR(64) NOT NULL,
    analysis_id BIGINT      NULL,
    -- from_state IS NULL 恰好等於「鏈誕生」，見 postgres 版註解
    from_state  VARCHAR(40) NULL,
    to_state    VARCHAR(40) NOT NULL,
    trigger_event_type VARCHAR(80) NULL,
    reason_codes TEXT       NOT NULL DEFAULT ('[]'),
    occurred_at DATETIME(0) NOT NULL,
    created_at  DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_event_transitions_event FOREIGN KEY (event_uid) REFERENCES event_instances(event_uid),
    INDEX idx_event_transitions_event (event_uid, occurred_at)
);

-- zone 身分歷來用過的 zone_key，key 解析的第二把鑰匙。每個 zone_uid 上限 8 筆
-- （prune 與寫入同一交易），完整理由見 postgres 版註解。
--
-- zone_key 進了 PRIMARY KEY，所以長度要吃得下 utf8mb4 的索引上限：
-- (64 + 96) * 4 = 640 bytes，遠低於 InnoDB 的 3072。
CREATE TABLE IF NOT EXISTS zone_key_aliases (
    zone_uid      VARCHAR(64) NOT NULL,
    zone_key      VARCHAR(96) NOT NULL,
    first_seen_at DATETIME(0) NOT NULL,
    last_seen_at  DATETIME(0) NOT NULL,
    PRIMARY KEY (zone_uid, zone_key),
    CONSTRAINT fk_zone_key_aliases_zone FOREIGN KEY (zone_uid) REFERENCES zone_instances(zone_uid),
    INDEX idx_zone_key_aliases_key (zone_key, last_seen_at)
);

-- +goose Down
DROP TABLE IF EXISTS zone_key_aliases;
DROP TABLE IF EXISTS event_transitions;
DROP TABLE IF EXISTS event_instances;
