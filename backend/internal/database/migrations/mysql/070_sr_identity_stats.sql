-- 身分關聯決策的逐次分析統計（todo.md T-050）。
-- 完整設計理由見 postgres 版的註解與 docs/database-schema.md。
--
-- mysql 差異：BIGINT AUTO_INCREMENT、TIMESTAMP 預設、索引長度需給定。
-- **本 engine 從未部署過**（見 docs/issue.md I-054），只由
-- scripts/test-mysql-migrations.sh 驗證 DDL。

-- +goose Up
CREATE TABLE IF NOT EXISTS sr_identity_stats (
    id          BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
    analysis_id BIGINT       NOT NULL,
    symbol      VARCHAR(10)  NOT NULL,
    timeframe   VARCHAR(8)   NOT NULL,

    matched_by_chain   INT NOT NULL DEFAULT 0,
    matched_by_current INT NOT NULL DEFAULT 0,
    matched_by_alias   INT NOT NULL DEFAULT 0,

    unmatched_keys       INT NOT NULL DEFAULT 0,
    carried_noop         INT NOT NULL DEFAULT 0,
    zone_ended_skipped   INT NOT NULL DEFAULT 0,
    chain_conflicts      INT NOT NULL DEFAULT 0,
    chain_key_ambiguous  INT NOT NULL DEFAULT 0,
    alias_ambiguous      INT NOT NULL DEFAULT 0,
    carried_parse_fail   INT NOT NULL DEFAULT 0,
    invariant_violations INT NOT NULL DEFAULT 0,

    zone_identity_degraded  BOOLEAN NOT NULL DEFAULT FALSE,
    event_identity_degraded BOOLEAN NOT NULL DEFAULT FALSE,
    zone_live_candidates    INT NOT NULL DEFAULT 0,
    zone_ended              INT NOT NULL DEFAULT 0,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_sr_identity_stats_symbol_created (symbol, created_at),
    INDEX idx_sr_identity_stats_analysis (analysis_id)
);

-- +goose Down
DROP TABLE IF EXISTS sr_identity_stats;
