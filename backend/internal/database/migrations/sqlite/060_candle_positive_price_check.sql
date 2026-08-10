-- 擋掉價格非正的 K 棒（見 docs/issue.md I-064）。
-- SQLite 沒有 ALTER TABLE ADD CONSTRAINT，只能重建表；Up / Down 都是完整的表重建，
-- 且都保留資料（與 017／018 那種破壞性 migration 不同，這裡沒有資料損失）。

-- +goose Up
-- +goose StatementBegin
CREATE TABLE candles_new (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol     TEXT    NOT NULL,
    timeframe  TEXT    NOT NULL,
    open       REAL    NOT NULL,
    high       REAL    NOT NULL,
    low        REAL    NOT NULL,
    close      REAL    NOT NULL,
    volume     INTEGER NOT NULL,
    amount     REAL    NOT NULL DEFAULT 0,
    ts         DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol, timeframe, ts),
    CONSTRAINT ck_candles_positive_price
        CHECK (open > 0 AND high > 0 AND low > 0 AND close > 0)
);
INSERT INTO candles_new (id, symbol, timeframe, open, high, low, close, volume, amount, ts, created_at)
    SELECT id, symbol, timeframe, open, high, low, close, volume, amount, ts, created_at FROM candles;
DROP TABLE candles;
ALTER TABLE candles_new RENAME TO candles;
CREATE INDEX IF NOT EXISTS idx_candles_symbol_tf_ts ON candles(symbol, timeframe, ts DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE candles_old (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol     TEXT    NOT NULL,
    timeframe  TEXT    NOT NULL,
    open       REAL    NOT NULL,
    high       REAL    NOT NULL,
    low        REAL    NOT NULL,
    close      REAL    NOT NULL,
    volume     INTEGER NOT NULL,
    amount     REAL    NOT NULL DEFAULT 0,
    ts         DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol, timeframe, ts)
);
INSERT INTO candles_old (id, symbol, timeframe, open, high, low, close, volume, amount, ts, created_at)
    SELECT id, symbol, timeframe, open, high, low, close, volume, amount, ts, created_at FROM candles;
DROP TABLE candles;
ALTER TABLE candles_old RENAME TO candles;
CREATE INDEX IF NOT EXISTS idx_candles_symbol_tf_ts ON candles(symbol, timeframe, ts DESC);
-- +goose StatementEnd
