-- 評估標的池（T-040 Step 5 / Phase 2）。規格見
-- docs/evaluation-universe-selection-plan.md 的「Step 5 執行計畫書」。
--
-- 這張表**與 watchlists 分離**：watchlists 驅動六個流程，把 131 檔塞進去會讓每一個都
-- 乘上約 12 倍。本表只有一個用途——每日盤後更新這批標的的日 K。
-- **不參與任何交易決策或狀態推導。**
--
-- bucket_edge_low / bucket_edge_high 是刻意的反正規化，理由見 postgres 版的註解。
--
-- sqlite 的 TEXT 沒有長度上限，所以沒有對應的「放寬欄寬」migration；
-- 這是既有決策不是缺漏（比照 063～065 的處理）。

-- +goose Up
CREATE TABLE IF NOT EXISTS evaluation_universe (
    id               INTEGER  PRIMARY KEY AUTOINCREMENT,
    symbol           TEXT     NOT NULL UNIQUE,
    bucket_hint      TEXT     NOT NULL,
    bucket_edge_low  REAL     NOT NULL,
    bucket_edge_high REAL     NOT NULL,
    universe_version TEXT     NOT NULL,
    universe_role    TEXT     NOT NULL DEFAULT 'primary',
    selected_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    source           TEXT     NOT NULL,
    active           BOOLEAN  NOT NULL DEFAULT 1,
    note             TEXT     NOT NULL DEFAULT '',
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_evaluation_universe_edges
        CHECK (bucket_edge_low > 0 AND bucket_edge_high > bucket_edge_low)
);
CREATE INDEX IF NOT EXISTS idx_evaluation_universe_active
    ON evaluation_universe(active, symbol);

-- +goose Down
DROP INDEX IF EXISTS idx_evaluation_universe_active;
DROP TABLE IF EXISTS evaluation_universe;
