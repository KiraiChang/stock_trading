-- +goose Up
CREATE TABLE IF NOT EXISTS signals (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol      TEXT    NOT NULL,
    signal_type TEXT    NOT NULL,
    direction   TEXT    NOT NULL,
    price       REAL    NOT NULL,
    volume      INTEGER NOT NULL,
    vol_ratio   REAL,
    resistance  REAL,
    support     REAL,
    trend       TEXT,
    note        TEXT,
    ts          DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_signals_symbol_ts ON signals(symbol, ts DESC);
CREATE INDEX IF NOT EXISTS idx_signals_ts ON signals(ts DESC);

-- +goose Down
DROP TABLE IF EXISTS signals;
