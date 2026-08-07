-- +goose Up
-- 機構級重新設計（見 Python backtest/modular/sr_scoring/scoring.py 開頭的
-- 完整說明）：欄位變動幅度太大（新增 net_score/confidence_level/
-- expected_gain/expected_loss/reward_risk_percentile/volume_confirmation/
-- zone_momentum/zone_direction/recent_validation/trading_score/
-- trading_recommendation，移除 avg_return_after_touch/volatility/
-- trend_strength/rejection_count/breakout_count 改名），直接砍掉重建比
-- 逐欄 ALTER 更乾淨；這是還沒進生產環境的功能，沒有既有資料需要保留。
DROP TABLE IF EXISTS stock_sr_zones;
DROP TABLE IF EXISTS stock_sr_zone_analyses;

CREATE TABLE stock_sr_zone_analyses (
    id                 INTEGER  PRIMARY KEY AUTOINCREMENT,
    symbol             TEXT     NOT NULL,
    timeframe          TEXT     NOT NULL,
    analyzed_at        DATETIME NOT NULL,
    current_price      REAL     NOT NULL,
    overall_trend      REAL     NOT NULL,
    overall_volatility REAL     NOT NULL,
    model_version      TEXT     NOT NULL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_stock_sr_zone_analyses_symbol ON stock_sr_zone_analyses(symbol, created_at DESC);

CREATE TABLE stock_sr_zones (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id             INTEGER NOT NULL,
    price_low               REAL    NOT NULL,
    price_high              REAL    NOT NULL,
    method                  TEXT    NOT NULL,
    role                    TEXT    NOT NULL,

    support_score           REAL    NOT NULL,
    resistance_score        REAL    NOT NULL,
    net_score               REAL    NOT NULL,
    net_score_label         TEXT    NOT NULL,

    confidence              REAL    NOT NULL,
    confidence_level        TEXT    NOT NULL,

    bounce_probability      REAL,
    break_probability       REAL,
    expected_gain           REAL,
    expected_loss           REAL,
    expected_value          REAL,
    risk_reward_ratio       REAL,
    reward_risk_percentile  REAL,

    relative_volume         REAL,
    volume_confirmation     TEXT,

    touch_count             INTEGER NOT NULL,
    reject_count            INTEGER NOT NULL,
    break_count             INTEGER NOT NULL,

    zone_momentum           REAL    NOT NULL,
    zone_direction          TEXT    NOT NULL,

    recent_validation       TEXT    NOT NULL,

    trading_score           REAL    NOT NULL,
    trading_recommendation  TEXT    NOT NULL,

    status                  TEXT    NOT NULL DEFAULT 'PENDING',
    broken_at               DATETIME,
    broken_price            REAL,
    FOREIGN KEY(analysis_id) REFERENCES stock_sr_zone_analyses(id)
);
CREATE INDEX idx_stock_sr_zones_analysis_id ON stock_sr_zones(analysis_id);

-- +goose Down
-- 重建「014 + 015 + 016」的最終結構，理由見 postgres 版同名 migration 的 Down 註解
-- 與 development-workflow.md 的「migration 的 Down 區塊也要能跑」。
-- **資料回不來**，只還原結構。
DROP TABLE IF EXISTS stock_sr_zones;
DROP TABLE IF EXISTS stock_sr_zone_analyses;

CREATE TABLE stock_sr_zone_analyses (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    symbol         TEXT     NOT NULL,
    timeframe      TEXT     NOT NULL,
    analyzed_at    DATETIME NOT NULL,
    current_price  REAL     NOT NULL,
    model_version  TEXT     NOT NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_stock_sr_zone_analyses_symbol ON stock_sr_zone_analyses(symbol, created_at DESC);

CREATE TABLE stock_sr_zones (
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
    -- 016 加的三欄
    confidence              REAL    NOT NULL DEFAULT 0,
    expected_value          REAL,
    risk_reward_ratio       REAL,
    FOREIGN KEY(analysis_id) REFERENCES stock_sr_zone_analyses(id)
);
CREATE INDEX idx_stock_sr_zones_analysis_id ON stock_sr_zones(analysis_id);
