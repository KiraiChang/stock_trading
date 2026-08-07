-- +goose Up
-- 機構級重新設計，見 sqlite 版本同名 migration 的說明。
DROP TABLE IF EXISTS stock_sr_zones;
DROP TABLE IF EXISTS stock_sr_zone_analyses;

CREATE TABLE stock_sr_zone_analyses (
    id                 BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol             VARCHAR(10)   NOT NULL,
    timeframe          VARCHAR(5)    NOT NULL,
    analyzed_at        DATETIME(0)   NOT NULL,
    current_price      DECIMAL(10,2) NOT NULL,
    overall_trend      DECIMAL(10,6) NOT NULL,
    overall_volatility DECIMAL(10,6) NOT NULL,
    model_version      VARCHAR(20)   NOT NULL,
    created_at         DATETIME(0)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_symbol_created (symbol, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE stock_sr_zones (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    analysis_id             BIGINT UNSIGNED NOT NULL,
    price_low               DECIMAL(10,2) NOT NULL,
    price_high              DECIMAL(10,2) NOT NULL,
    method                  VARCHAR(20)   NOT NULL,
    role                    VARCHAR(15)   NOT NULL,

    support_score           DECIMAL(6,4)  NOT NULL,
    resistance_score        DECIMAL(6,4)  NOT NULL,
    net_score               DECIMAL(7,4)  NOT NULL,
    net_score_label         VARCHAR(20)   NOT NULL,

    confidence              DECIMAL(6,4)  NOT NULL,
    confidence_level        VARCHAR(15)   NOT NULL,

    bounce_probability      DECIMAL(6,4),
    break_probability       DECIMAL(6,4),
    expected_gain           DECIMAL(10,6),
    expected_loss           DECIMAL(10,6),
    expected_value          DECIMAL(10,6),
    risk_reward_ratio       DECIMAL(10,4),
    reward_risk_percentile  DECIMAL(6,2),

    relative_volume         DECIMAL(10,4),
    volume_confirmation     VARCHAR(15),

    touch_count             INT           NOT NULL,
    reject_count            INT           NOT NULL,
    break_count             INT           NOT NULL,

    zone_momentum           DECIMAL(10,6) NOT NULL,
    zone_direction          VARCHAR(10)   NOT NULL,

    recent_validation       VARCHAR(25)   NOT NULL,

    trading_score           DECIMAL(6,2)  NOT NULL,
    trading_recommendation  VARCHAR(15)   NOT NULL,

    status                  VARCHAR(15)   NOT NULL DEFAULT 'PENDING',
    broken_at               DATETIME(0),
    broken_price            DECIMAL(10,2),
    FOREIGN KEY (analysis_id) REFERENCES stock_sr_zone_analyses(id),
    INDEX idx_analysis_id (analysis_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
-- 重建「014 + 015 + 016」的最終結構，理由見 postgres 版同名 migration 的 Down 註解
-- 與 development-workflow.md 的「migration 的 Down 區塊也要能跑」。
-- **資料回不來**，只還原結構。
DROP TABLE IF EXISTS stock_sr_zones;
DROP TABLE IF EXISTS stock_sr_zone_analyses;

CREATE TABLE stock_sr_zone_analyses (
    id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol         VARCHAR(10)   NOT NULL,
    timeframe      VARCHAR(5)    NOT NULL,
    analyzed_at    DATETIME(0)   NOT NULL,
    current_price  DECIMAL(10,2) NOT NULL,
    model_version  VARCHAR(20)   NOT NULL,
    created_at     DATETIME(0)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_symbol_created (symbol, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE stock_sr_zones (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    analysis_id             BIGINT UNSIGNED NOT NULL,
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
    broken_at               DATETIME(0),
    broken_price            DECIMAL(10,2),
    -- 016 加的三欄
    confidence              DECIMAL(6,4)  NOT NULL DEFAULT 0,
    expected_value          DECIMAL(10,6),
    risk_reward_ratio       DECIMAL(10,4),
    FOREIGN KEY (analysis_id) REFERENCES stock_sr_zone_analyses(id),
    INDEX idx_analysis_id (analysis_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
