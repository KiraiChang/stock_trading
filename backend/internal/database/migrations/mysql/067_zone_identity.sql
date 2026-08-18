-- Zone 身分、一世與轉換（T-048 階段 B）。完整設計理由見 postgres 版的註解與
-- docs/todo.md T-048。
--
-- mysql 從未在任何環境部署，唯一的驗證路徑是 scripts/test-mysql-migrations.sh（見 issue.md I-054）。
--
-- mysql 差異：
--   * TIMESTAMPTZ → DATETIME(0)（比照既有 migration，本專案不用 mysql 的 TIMESTAMP）
--   * BIGSERIAL → BIGINT UNSIGNED AUTO_INCREMENT
--   * DROP INDEX 沒有 IF EXISTS，但 Down 直接 DROP TABLE 就會一併移除索引
--   * reason_codes 用 TEXT DEFAULT '(...)' 的括號寫法（TEXT 的 DEFAULT 需要表達式語法）
--
-- 欄位名都避開了 MySQL 保留字（見 database-schema.md「欄位命名規範」）；
-- role / state / relation / seq 都是非保留字，且 role 已在 stock_sr_zones 用過。

-- +goose Up

CREATE TABLE IF NOT EXISTS zone_instances (
    zone_uid      VARCHAR(64)   NOT NULL PRIMARY KEY,
    symbol        VARCHAR(10)   NOT NULL,
    timeframe     VARCHAR(8)    NOT NULL,
    method        VARCHAR(32)   NOT NULL,
    -- ACTIVE / SPLIT / MERGED / RESHAPED（後三者為身分終態）
    state         VARCHAR(16)   NOT NULL DEFAULT 'ACTIVE',
    price_low     DECIMAL(18,6) NOT NULL,
    price_high    DECIMAL(18,6) NOT NULL,
    first_seen_at DATETIME(0)   NOT NULL,
    last_seen_at  DATETIME(0)   NOT NULL,
    -- 資格閘門的第二個軸，見 postgres 版註解
    observed_absences INT     NOT NULL DEFAULT 0,
    ended_at      DATETIME(0)   NULL,
    created_at    DATETIME(0)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME(0)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_zone_instances_prices CHECK (price_high >= price_low),
    INDEX idx_zone_instances_live (symbol, timeframe, state, last_seen_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS zone_role_incarnations (
    incarnation_uid VARCHAR(64) NOT NULL PRIMARY KEY,
    zone_uid        VARCHAR(64) NOT NULL,
    seq             INT         NOT NULL,
    -- 只收 SUPPORT / RESISTANCE；AT_ZONE 不開一世（見 postgres 版註解）
    role            VARCHAR(16) NOT NULL,
    -- ACTIVE / TESTING / INVALIDATED / EXPIRED（見 postgres 版註解）
    state           VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    started_at      DATETIME(0) NOT NULL,
    ended_at        DATETIME(0) NULL,
    expired_at      DATETIME(0) NULL,
    end_reason      VARCHAR(32) NULL,
    created_at      DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_zone_incarnation_seq UNIQUE (zone_uid, seq),
    CONSTRAINT fk_zone_incarnation_zone FOREIGN KEY (zone_uid)
        REFERENCES zone_instances(zone_uid),
    INDEX idx_zone_incarnations_open (zone_uid, ended_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS zone_transitions (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    zone_uid        VARCHAR(64) NOT NULL,
    incarnation_uid VARCHAR(64) NULL,
    analysis_id     BIGINT      NULL,
    -- STATE_CHANGE / ROLE_RESOLVED / ROLE_UNRESOLVED / ROLE_FLIPPED
    transition_kind VARCHAR(24) NOT NULL,
    from_state      VARCHAR(16) NULL,
    to_state        VARCHAR(16) NULL,
    from_role       VARCHAR(16) NULL,
    to_role         VARCHAR(16) NULL,
    -- 不合法的轉換照樣寫入，只標記不擋（見 postgres 版註解）
    is_illegal      BOOLEAN     NOT NULL DEFAULT FALSE,
    reason_codes    TEXT        NOT NULL DEFAULT ('[]'),
    occurred_at     DATETIME(0) NOT NULL,
    created_at      DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_zone_transitions_zone FOREIGN KEY (zone_uid)
        REFERENCES zone_instances(zone_uid),
    CONSTRAINT fk_zone_transitions_incarnation FOREIGN KEY (incarnation_uid)
        REFERENCES zone_role_incarnations(incarnation_uid),
    INDEX idx_zone_transitions_zone (zone_uid, occurred_at),
    INDEX idx_zone_transitions_illegal (is_illegal, occurred_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS zone_relations (
    parent_zone_uid VARCHAR(64) NOT NULL,
    child_zone_uid  VARCHAR(64) NOT NULL,
    -- SPLIT / MERGE / RESHAPE。**沒有 CONTINUE**，自環會讓祖先回溯無法終止。
    relation        VARCHAR(16) NOT NULL,
    analysis_id     BIGINT      NULL,
    occurred_at     DATETIME(0) NOT NULL,
    created_at      DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (parent_zone_uid, child_zone_uid, occurred_at),
    CONSTRAINT ck_zone_relations_no_self_loop CHECK (parent_zone_uid <> child_zone_uid),
    CONSTRAINT fk_zone_relations_parent FOREIGN KEY (parent_zone_uid)
        REFERENCES zone_instances(zone_uid),
    CONSTRAINT fk_zone_relations_child FOREIGN KEY (child_zone_uid)
        REFERENCES zone_instances(zone_uid),
    INDEX idx_zone_relations_child (child_zone_uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
-- 順序由外鍵決定：先掉引用方，再掉被引用方。
DROP TABLE IF EXISTS zone_relations;
DROP TABLE IF EXISTS zone_transitions;
DROP TABLE IF EXISTS zone_role_incarnations;
DROP TABLE IF EXISTS zone_instances;
