-- +goose Up
CREATE TABLE position_transactions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, symbol VARCHAR(20) NOT NULL,
    event_type VARCHAR(24) NOT NULL, occurred_at DATETIME(0) NOT NULL,
    shares DECIMAL(18,4), price DECIMAL(18,4), fee DECIMAL(18,4) NOT NULL DEFAULT 0,
    tax DECIMAL(18,4) NOT NULL DEFAULT 0, target_shares DECIMAL(18,4),
    target_avg_cost DECIMAL(18,4), note TEXT NOT NULL,
    created_at DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_position_transactions_symbol_time(symbol,occurred_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE positions (
    symbol VARCHAR(20) PRIMARY KEY, shares DECIMAL(18,4) NOT NULL,
    avg_cost DECIMAL(18,4) NOT NULL, realized_pnl DECIMAL(18,4) NOT NULL DEFAULT 0,
    version BIGINT NOT NULL, last_event_id BIGINT UNSIGNED NOT NULL,
    updated_at DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_positions_last_event
        FOREIGN KEY (last_event_id) REFERENCES position_transactions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE position_analyses (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, symbol VARCHAR(20) NOT NULL,
    position_state VARCHAR(12) NOT NULL, position_version BIGINT NOT NULL,
    shares DECIMAL(18,4) NOT NULL, avg_cost DECIMAL(18,4) NOT NULL,
    realized_pnl DECIMAL(18,4) NOT NULL, analyzed_at DATETIME(0) NOT NULL,
    current_price DECIMAL(18,4) NOT NULL, sr_zone_analysis_id BIGINT UNSIGNED,
    action VARCHAR(32) NOT NULL, action_label VARCHAR(64) NOT NULL,
    target_shares DECIMAL(18,4) NOT NULL, adjustment_shares DECIMAL(18,4) NOT NULL,
    adjustment_side VARCHAR(8) NOT NULL, adjustment_amount DECIMAL(18,4) NOT NULL,
    entry_price DECIMAL(18,4), stop_loss_price DECIMAL(18,4), take_profit_price DECIMAL(18,4),
    risk_amount DECIMAL(18,4), expected_reward_amount DECIMAL(18,4), risk_reward_ratio DECIMAL(18,6),
    unrealized_pnl DECIMAL(18,4) NOT NULL, unrealized_pnl_pct DECIMAL(18,6) NOT NULL,
    config_json LONGTEXT NOT NULL, reason LONGTEXT NOT NULL, evidence LONGTEXT NOT NULL,
    trigger_conditions LONGTEXT NOT NULL, invalidation_conditions LONGTEXT NOT NULL,
    rule_version VARCHAR(64) NOT NULL, created_at DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_position_analyses_symbol(symbol,created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT INTO position_transactions(symbol,event_type,occurred_at,shares,price,note)
SELECT symbol,'OPENING_BALANCE',MIN(created_at),SUM(shares),
       SUM(shares*cost_price)/SUM(shares),'Migrated from holdings'
FROM holdings GROUP BY symbol HAVING SUM(shares)>0;
INSERT INTO positions(symbol,shares,avg_cost,realized_pnl,version,last_event_id)
SELECT symbol,shares,price,0,1,id FROM position_transactions WHERE event_type='OPENING_BALANCE';
INSERT INTO position_analyses(
    symbol,position_state,position_version,shares,avg_cost,realized_pnl,analyzed_at,current_price,
    sr_zone_analysis_id,action,action_label,target_shares,adjustment_shares,adjustment_side,
    adjustment_amount,stop_loss_price,take_profit_price,unrealized_pnl,unrealized_pnl_pct,
    config_json,reason,evidence,trigger_conditions,invalidation_conditions,rule_version,created_at)
SELECT symbol,IF(shares>0,'LONG','FLAT'),0,shares,cost_price,0,analyzed_at,current_price,
       sr_zone_analysis_id,action,action_label,shares,0,'NONE',0,stop_loss_price,take_profit_price,
       unrealized_pnl,unrealized_pnl_pct,detail_json,reason,detail_json,'[]','[]',
       'holding_sr_zone_v1_legacy',created_at FROM holding_analyses;
DROP TABLE holding_analyses;
DROP TABLE holdings;
-- +goose Down
DROP TABLE position_analyses;
DROP TABLE positions;
DROP TABLE position_transactions;
