-- 身分關聯決策的逐次分析統計（todo.md T-050）。
-- 完整設計理由見 postgres 版的註解與 docs/database-schema.md。
--
-- sqlite 差異：INTEGER PRIMARY KEY AUTOINCREMENT、BOOLEAN 以 0/1 存、時間用 TEXT/TIMESTAMP。

-- +goose Up
CREATE TABLE IF NOT EXISTS sr_identity_stats (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id INTEGER NOT NULL,
    symbol      TEXT    NOT NULL,
    timeframe   TEXT    NOT NULL,

    matched_by_chain   INTEGER NOT NULL DEFAULT 0,
    matched_by_current INTEGER NOT NULL DEFAULT 0,
    matched_by_alias   INTEGER NOT NULL DEFAULT 0,

    unmatched_keys       INTEGER NOT NULL DEFAULT 0,
    carried_noop         INTEGER NOT NULL DEFAULT 0,
    zone_ended_skipped   INTEGER NOT NULL DEFAULT 0,
    chain_conflicts      INTEGER NOT NULL DEFAULT 0,
    chain_key_ambiguous  INTEGER NOT NULL DEFAULT 0,
    alias_ambiguous      INTEGER NOT NULL DEFAULT 0,
    carried_parse_fail   INTEGER NOT NULL DEFAULT 0,
    invariant_violations INTEGER NOT NULL DEFAULT 0,

    zone_identity_degraded  BOOLEAN NOT NULL DEFAULT 0,
    event_identity_degraded BOOLEAN NOT NULL DEFAULT 0,
    zone_live_candidates    INTEGER NOT NULL DEFAULT 0,
    zone_ended              INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sr_identity_stats_symbol_created
    ON sr_identity_stats (symbol, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sr_identity_stats_analysis
    ON sr_identity_stats (analysis_id);

-- +goose Down
DROP TABLE IF EXISTS sr_identity_stats;
