-- Zone 身分、一世與轉換（T-048 階段 B）。完整設計理由見 postgres 版的註解與
-- docs/sr-zone-scoring.md「Zone 身分與 ZoneMatcher」。
--
-- sqlite 差異：TEXT 沒有長度上限、沒有 JSONB（用 TEXT 存 JSON 字串）、
-- 沒有部分索引以外的差別。BIGSERIAL → INTEGER PRIMARY KEY AUTOINCREMENT。

-- +goose Up

CREATE TABLE IF NOT EXISTS zone_instances (
    zone_uid      TEXT    PRIMARY KEY,
    symbol        TEXT    NOT NULL,
    timeframe     TEXT    NOT NULL,
    method        TEXT    NOT NULL,
    -- ACTIVE / SPLIT / MERGED / RESHAPED（後三者為身分終態）
    state         TEXT    NOT NULL DEFAULT 'ACTIVE',
    price_low     REAL    NOT NULL,
    price_high    REAL    NOT NULL,
    first_seen_at TIMESTAMP NOT NULL,
    last_seen_at  TIMESTAMP NOT NULL,
    -- 資格閘門的第二個軸，見 postgres 版註解
    observed_absences INTEGER NOT NULL DEFAULT 0,
    -- 上次觀測到的 role，與一世的角色是兩回事（見 postgres 版註解）
    last_role     TEXT    NOT NULL DEFAULT 'AT_ZONE',
    ended_at      TIMESTAMP,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (price_high >= price_low)
);
CREATE INDEX IF NOT EXISTS idx_zone_instances_live
    ON zone_instances (symbol, timeframe, state, last_seen_at);

CREATE TABLE IF NOT EXISTS zone_role_incarnations (
    incarnation_uid TEXT    PRIMARY KEY,
    zone_uid        TEXT    NOT NULL REFERENCES zone_instances(zone_uid),
    seq             INTEGER NOT NULL,
    -- 只收 SUPPORT / RESISTANCE；AT_ZONE 不開一世（見 postgres 版註解）
    role            TEXT    NOT NULL,
    -- ACTIVE / TESTING / INVALIDATED / EXPIRED（見 postgres 版註解）
    state           TEXT    NOT NULL DEFAULT 'ACTIVE',
    started_at      TIMESTAMP NOT NULL,
    ended_at        TIMESTAMP,
    expired_at      TIMESTAMP,
    end_reason      TEXT,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (zone_uid, seq)
);
CREATE INDEX IF NOT EXISTS idx_zone_incarnations_open
    ON zone_role_incarnations (zone_uid, ended_at);

CREATE TABLE IF NOT EXISTS zone_transitions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_uid        TEXT    NOT NULL REFERENCES zone_instances(zone_uid),
    incarnation_uid TEXT    REFERENCES zone_role_incarnations(incarnation_uid),
    analysis_id     INTEGER,
    -- STATE_CHANGE / ROLE_RESOLVED / ROLE_UNRESOLVED / ROLE_FLIPPED
    transition_kind TEXT    NOT NULL,
    from_state      TEXT,
    to_state        TEXT,
    from_role       TEXT,
    to_role         TEXT,
    -- 不合法的轉換照樣寫入，只標記不擋（見 postgres 版註解）
    is_illegal      BOOLEAN NOT NULL DEFAULT 0,
    reason_codes    TEXT    NOT NULL DEFAULT '[]',
    occurred_at     TIMESTAMP NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_zone_transitions_zone
    ON zone_transitions (zone_uid, occurred_at);
CREATE INDEX IF NOT EXISTS idx_zone_transitions_illegal
    ON zone_transitions (is_illegal, occurred_at);

CREATE TABLE IF NOT EXISTS zone_relations (
    parent_zone_uid TEXT NOT NULL REFERENCES zone_instances(zone_uid),
    child_zone_uid  TEXT NOT NULL REFERENCES zone_instances(zone_uid),
    -- SPLIT / MERGE / RESHAPE。**沒有 CONTINUE**，自環會讓祖先回溯無法終止。
    relation        TEXT NOT NULL,
    analysis_id     INTEGER,
    occurred_at     TIMESTAMP NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (parent_zone_uid, child_zone_uid, occurred_at),
    CHECK (parent_zone_uid <> child_zone_uid)
);
CREATE INDEX IF NOT EXISTS idx_zone_relations_child
    ON zone_relations (child_zone_uid);

-- +goose Down
DROP INDEX IF EXISTS idx_zone_relations_child;
DROP TABLE IF EXISTS zone_relations;
DROP INDEX IF EXISTS idx_zone_transitions_illegal;
DROP INDEX IF EXISTS idx_zone_transitions_zone;
DROP TABLE IF EXISTS zone_transitions;
DROP INDEX IF EXISTS idx_zone_incarnations_open;
DROP TABLE IF EXISTS zone_role_incarnations;
DROP INDEX IF EXISTS idx_zone_instances_live;
DROP TABLE IF EXISTS zone_instances;
