-- +goose Up
CREATE TABLE IF NOT EXISTS stock_symbols (
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

CREATE INDEX IF NOT EXISTS idx_stock_symbols_is_listed ON stock_symbols(is_listed);
CREATE INDEX IF NOT EXISTS idx_stock_symbols_security_type ON stock_symbols(security_type);

-- +goose Down
DROP TABLE IF EXISTS stock_symbols;
