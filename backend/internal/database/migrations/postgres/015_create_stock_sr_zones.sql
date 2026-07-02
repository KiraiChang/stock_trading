-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_zones (
    id                      BIGSERIAL     PRIMARY KEY,
    analysis_id             BIGINT        NOT NULL REFERENCES stock_sr_zone_analyses(id),
    price_low               DECIMAL(10,2) NOT NULL,
    price_high              DECIMAL(10,2) NOT NULL,
    method                  VARCHAR(20)   NOT NULL,
    role                    VARCHAR(15)   NOT NULL,
    support_score           DECIMAL(6,4)  NOT NULL,
    resistance_score        DECIMAL(6,4)  NOT NULL,
    bounce_probability      DECIMAL(6,4),
    break_probability       DECIMAL(6,4),
    touch_count             INT           NOT NULL,
    rejection_count         INT           NOT NULL,
    breakout_count          INT           NOT NULL,
    avg_return_after_touch  DECIMAL(10,6) NOT NULL,
    relative_volume         DECIMAL(10,6) NOT NULL,
    volatility              DECIMAL(10,6) NOT NULL,
    trend_strength          DECIMAL(10,6) NOT NULL,
    status                  VARCHAR(15)   NOT NULL DEFAULT 'PENDING',
    broken_at               TIMESTAMPTZ,
    broken_price            DECIMAL(10,2)
);

CREATE INDEX IF NOT EXISTS idx_stock_sr_zones_analysis_id ON stock_sr_zones (analysis_id);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_zones;
