CREATE TABLE IF NOT EXISTS indicator_snapshots (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol      TEXT    NOT NULL,
    timeframe   TEXT    NOT NULL,
    ts          DATETIME NOT NULL,
    ma5         REAL,
    ma10        REAL,
    ma20        REAL,
    ma60        REAL,
    rsi14       REAL,
    macd        REAL,
    macd_signal REAL,
    macd_hist   REAL,
    bb_upper    REAL,
    bb_middle   REAL,
    bb_lower    REAL,
    atr14       REAL,
    vwap        REAL,
    vol_ma20    INTEGER,
    vol_ratio   REAL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol, timeframe, ts)
);

CREATE INDEX IF NOT EXISTS idx_indicators_symbol_tf
    ON indicator_snapshots(symbol, timeframe, ts DESC);
