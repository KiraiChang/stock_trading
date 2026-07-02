-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_zones (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id             INTEGER NOT NULL,
    price_low               REAL    NOT NULL,
    price_high              REAL    NOT NULL,
    method                  TEXT    NOT NULL,
    role                    TEXT    NOT NULL,
    support_score           REAL    NOT NULL,
    resistance_score        REAL    NOT NULL,
    bounce_probability      REAL,
    break_probability       REAL,
    touch_count             INTEGER NOT NULL,
    rejection_count         INTEGER NOT NULL,
    breakout_count          INTEGER NOT NULL,
    avg_return_after_touch  REAL    NOT NULL,
    relative_volume         REAL    NOT NULL,
    volatility              REAL    NOT NULL,
    trend_strength          REAL    NOT NULL,
    status                  TEXT    NOT NULL DEFAULT 'PENDING',
    broken_at               DATETIME,
    broken_price            REAL,
    FOREIGN KEY(analysis_id) REFERENCES stock_sr_zone_analyses(id)
);

CREATE INDEX IF NOT EXISTS idx_stock_sr_zones_analysis_id ON stock_sr_zones(analysis_id);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_zones;
