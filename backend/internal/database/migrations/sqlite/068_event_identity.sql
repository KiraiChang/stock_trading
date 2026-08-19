-- 事件身分與生命週期（T-048 階段 C）。完整設計理由見 postgres 版的註解與
-- docs/sr-zone-scoring.md「Zone 身分與 ZoneMatcher」。
--
-- sqlite 差異：TEXT 沒有長度上限、BIGSERIAL → INTEGER PRIMARY KEY AUTOINCREMENT。

-- +goose Up

CREATE TABLE IF NOT EXISTS event_instances (
    event_uid      TEXT    PRIMARY KEY,
    symbol         TEXT    NOT NULL,
    timeframe      TEXT    NOT NULL,
    -- 可為 NULL：SYMBOL scope 的事件不屬於任何 zone，見 postgres 版註解
    zone_uid       TEXT    NULL REFERENCES zone_instances(zone_uid),
    -- zone_uid 的 NOT NULL 投影，ZONE scope 時等於 zone_uid、SYMBOL scope 時是 'SYMBOL'
    zone_scope_key TEXT    NOT NULL,
    event_scope    TEXT    NOT NULL,
    event_family   TEXT    NOT NULL,
    -- 同一個 (zone, family) 的第幾條鏈，比照 zone_role_incarnations.seq
    seq            INTEGER NOT NULL DEFAULT 1,
    root_event_type   TEXT NOT NULL,
    latest_event_type TEXT NOT NULL,
    state          TEXT    NOT NULL,
    active         BOOLEAN NOT NULL DEFAULT 0,
    direction      TEXT    NOT NULL,
    resolved_by    TEXT    NULL,
    first_seen_at  TIMESTAMP NOT NULL,
    last_seen_at   TIMESTAMP NOT NULL,
    ended_at       TIMESTAMP NULL,
    -- 這條鏈最近一次被觀測到時事件身上帶的 zone_key。鏈延續的第一把鑰匙，
    -- 見 postgres 版註解
    last_zone_key  TEXT    NULL,
    -- RESOLVED / EXPIRED / ZONE_IDENTITY_ENDED
    end_reason     TEXT    NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_event_instance_seq UNIQUE (symbol, timeframe, zone_scope_key, event_family, seq)
);
CREATE INDEX IF NOT EXISTS idx_event_instances_live
    ON event_instances (symbol, timeframe, ended_at, last_seen_at);
CREATE INDEX IF NOT EXISTS idx_event_instances_zone
    ON event_instances (zone_uid);
-- 第一把鑰匙的查找路徑：(last_zone_key, event_family) → 活鏈。
CREATE INDEX IF NOT EXISTS idx_event_instances_last_zone_key
    ON event_instances (symbol, timeframe, last_zone_key, event_family);

CREATE TABLE IF NOT EXISTS event_transitions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    event_uid   TEXT    NOT NULL REFERENCES event_instances(event_uid),
    analysis_id INTEGER NULL,
    -- from_state IS NULL 恰好等於「鏈誕生」，見 postgres 版註解
    from_state  TEXT    NULL,
    to_state    TEXT    NOT NULL,
    trigger_event_type TEXT NULL,
    reason_codes TEXT   NOT NULL DEFAULT '[]',
    occurred_at TIMESTAMP NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_event_transitions_event
    ON event_transitions (event_uid, occurred_at);

-- zone 身分歷來用過的 zone_key，key 解析的第二把鑰匙。每個 zone_uid 上限 8 筆
-- （prune 與寫入同一交易），完整理由見 postgres 版註解。
CREATE TABLE IF NOT EXISTS zone_key_aliases (
    zone_uid      TEXT      NOT NULL REFERENCES zone_instances(zone_uid),
    zone_key      TEXT      NOT NULL,
    first_seen_at TIMESTAMP NOT NULL,
    last_seen_at  TIMESTAMP NOT NULL,
    PRIMARY KEY (zone_uid, zone_key)
);
CREATE INDEX IF NOT EXISTS idx_zone_key_aliases_key
    ON zone_key_aliases (zone_key, last_seen_at);

-- +goose Down
DROP INDEX IF EXISTS idx_zone_key_aliases_key;
DROP TABLE IF EXISTS zone_key_aliases;
DROP INDEX IF EXISTS idx_event_instances_last_zone_key;
DROP INDEX IF EXISTS idx_event_transitions_event;
DROP TABLE IF EXISTS event_transitions;
DROP INDEX IF EXISTS idx_event_instances_zone;
DROP INDEX IF EXISTS idx_event_instances_live;
DROP TABLE IF EXISTS event_instances;
