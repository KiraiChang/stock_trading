-- 終止上市名單的事件層與投影 provenance。設計理由見同編號的 postgres 檔。
--
-- ⛔ **SQLite 沒有 ALTER TABLE ADD CONSTRAINT**，所以加 FK 與 CHECK 只能**重建表**
-- （與 migration 060 同一手法）。Up / Down 都是完整重建且**保留資料**——
-- 這裡沒有資料損失，與 017／018 那種破壞性 migration 不同。
--
-- ⚠️ 重建期間要關掉 foreign_keys：DROP 舊表時若已有其他表指向它會被擋。
-- goose 的每個 statement 各自執行，PRAGMA 又是 connection-local，
-- 所以用 legacy_alter_table 之外的作法：先建新表、搬資料、刪舊表、改名。
-- stock_symbols 目前沒有任何表以 FK 指向它（全 repo 只有 048 建它），
-- 所以直接 DROP 是安全的。

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS delisting_events (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    source                 TEXT     NOT NULL,
    symbol                 TEXT     NOT NULL,
    company_name           TEXT     NOT NULL,
    delisted_date          DATETIME NOT NULL,
    first_seen_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- 非 NULL = 已從來源消失，不參與投影但保留供人工判讀。
    missing_from_source_at DATETIME,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source, symbol, delisted_date)
);
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS idx_delisting_events_symbol ON delisting_events(symbol);

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS delisting_source_snapshots (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    source       TEXT     NOT NULL,
    -- nullable + ON DELETE SET NULL：job_runs 只留 30 天，而快照是縮水基準、
    -- 必須長期保存。CASCADE 會把基準一起刪掉。
    job_run_id   INTEGER REFERENCES job_runs(id) ON DELETE SET NULL,
    fetched_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    row_count    INTEGER  NOT NULL,
    content_hash TEXT     NOT NULL,
    accepted     INTEGER  NOT NULL DEFAULT 1,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS idx_delisting_snapshots_source_id
    ON delisting_source_snapshots(source, id DESC);

-- 重建 stock_symbols 以加上兩個新欄位、FK（RESTRICT）與 CHECK。
-- +goose StatementBegin
CREATE TABLE stock_symbols_new (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol        TEXT NOT NULL,
    name          TEXT NOT NULL,
    isin_code     TEXT NOT NULL DEFAULT '',
    market        TEXT NOT NULL DEFAULT '',
    security_type TEXT NOT NULL DEFAULT '',
    industry      TEXT NOT NULL DEFAULT '',
    cfi_code      TEXT NOT NULL DEFAULT '',
    remarks       TEXT NOT NULL DEFAULT '',
    listed_date   DATETIME,
    is_listed     INTEGER NOT NULL DEFAULT 1,
    last_seen_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delisted_date DATETIME,
    -- ⛔ RESTRICT 而不是 SET NULL：它是**所有權標記**，被清成 NULL 會讓 job-owned
    -- 的投影被誤認成人工值，從此不參與重算、永久無法收斂。
    delisted_event_id INTEGER REFERENCES delisting_events(id) ON DELETE RESTRICT,
    UNIQUE(symbol),
    -- 擋半套狀態：指向事件卻沒有日期，那是只清一欄的實作錯誤訊號。
    CONSTRAINT ck_stock_symbols_delisted_pair
        CHECK (delisted_event_id IS NULL OR delisted_date IS NOT NULL)
);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO stock_symbols_new
    (id, symbol, name, isin_code, market, security_type, industry, cfi_code,
     remarks, listed_date, is_listed, last_seen_at, created_at, updated_at)
SELECT id, symbol, name, isin_code, market, security_type, industry, cfi_code,
       remarks, listed_date, is_listed, last_seen_at, created_at, updated_at
FROM stock_symbols;
-- +goose StatementEnd

DROP TABLE stock_symbols;
ALTER TABLE stock_symbols_new RENAME TO stock_symbols;
CREATE INDEX IF NOT EXISTS idx_stock_symbols_is_listed ON stock_symbols(is_listed);
CREATE INDEX IF NOT EXISTS idx_stock_symbols_security_type ON stock_symbols(security_type);

-- +goose Down
-- 順序與 Up 相反：先把 stock_symbols 還原成沒有那兩欄的樣子（連 FK 一起消失），
-- 再刪事件表。反過來做的話 DROP delisting_events 會被 FK 擋住。
-- +goose StatementBegin
CREATE TABLE stock_symbols_old (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol        TEXT NOT NULL,
    name          TEXT NOT NULL,
    isin_code     TEXT NOT NULL DEFAULT '',
    market        TEXT NOT NULL DEFAULT '',
    security_type TEXT NOT NULL DEFAULT '',
    industry      TEXT NOT NULL DEFAULT '',
    cfi_code      TEXT NOT NULL DEFAULT '',
    remarks       TEXT NOT NULL DEFAULT '',
    listed_date   DATETIME,
    is_listed     INTEGER NOT NULL DEFAULT 1,
    last_seen_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol)
);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO stock_symbols_old
    (id, symbol, name, isin_code, market, security_type, industry, cfi_code,
     remarks, listed_date, is_listed, last_seen_at, created_at, updated_at)
SELECT id, symbol, name, isin_code, market, security_type, industry, cfi_code,
       remarks, listed_date, is_listed, last_seen_at, created_at, updated_at
FROM stock_symbols;
-- +goose StatementEnd

DROP TABLE stock_symbols;
ALTER TABLE stock_symbols_old RENAME TO stock_symbols;
CREATE INDEX IF NOT EXISTS idx_stock_symbols_is_listed ON stock_symbols(is_listed);
CREATE INDEX IF NOT EXISTS idx_stock_symbols_security_type ON stock_symbols(security_type);

DROP TABLE IF EXISTS delisting_source_snapshots;
DROP TABLE IF EXISTS delisting_events;
