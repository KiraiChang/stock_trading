-- +goose Up
CREATE TABLE IF NOT EXISTS holdings (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol      VARCHAR(20)    NOT NULL,
    shares      DECIMAL(18,4)  NOT NULL,
    cost_price  DECIMAL(18,4)  NOT NULL,
    note        TEXT           NOT NULL DEFAULT (''),
    created_at  DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_holdings_symbol (symbol)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS holding_analyses (
    id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    holding_id           BIGINT UNSIGNED NOT NULL,
    symbol               VARCHAR(20)    NOT NULL,
    shares               DECIMAL(18,4)  NOT NULL,
    cost_price           DECIMAL(18,4)  NOT NULL,
    analyzed_at          DATETIME(0)    NOT NULL,
    current_price        DECIMAL(18,4)  NOT NULL,
    sr_zone_analysis_id  BIGINT UNSIGNED,
    action               VARCHAR(32)    NOT NULL,
    action_label         VARCHAR(64)    NOT NULL,
    stop_loss_price      DECIMAL(18,4),
    stop_loss_amount     DECIMAL(18,4),
    take_profit_price    DECIMAL(18,4),
    take_profit_amount   DECIMAL(18,4),
    add_on_trigger_price DECIMAL(18,4),
    add_on_amount        DECIMAL(18,4),
    unrealized_pnl       DECIMAL(18,4)  NOT NULL,
    unrealized_pnl_pct   DECIMAL(18,6)  NOT NULL,
    reason               TEXT           NOT NULL DEFAULT ('[]'),
    detail_json          TEXT           NOT NULL DEFAULT ('{}'),
    created_at           DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_holding_analyses_holding_id (holding_id, created_at DESC),
    INDEX idx_holding_analyses_symbol (symbol, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS holding_analyses;
DROP TABLE IF EXISTS holdings;
