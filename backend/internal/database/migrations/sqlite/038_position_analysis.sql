-- +goose Up
CREATE TABLE position_transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT, symbol TEXT NOT NULL, event_type TEXT NOT NULL,
    occurred_at DATETIME NOT NULL, shares REAL, price REAL, fee REAL NOT NULL DEFAULT 0,
    tax REAL NOT NULL DEFAULT 0, target_shares REAL, target_avg_cost REAL,
    note TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_position_transactions_symbol_time ON position_transactions(symbol, occurred_at, id);
CREATE TABLE positions (
    symbol TEXT PRIMARY KEY, shares REAL NOT NULL, avg_cost REAL NOT NULL,
    realized_pnl REAL NOT NULL DEFAULT 0, version INTEGER NOT NULL,
    last_event_id INTEGER NOT NULL, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE position_analyses (
    id INTEGER PRIMARY KEY AUTOINCREMENT, symbol TEXT NOT NULL, position_state TEXT NOT NULL,
    position_version INTEGER NOT NULL, shares REAL NOT NULL, avg_cost REAL NOT NULL,
    realized_pnl REAL NOT NULL, analyzed_at DATETIME NOT NULL, current_price REAL NOT NULL,
    sr_zone_analysis_id INTEGER, action TEXT NOT NULL, action_label TEXT NOT NULL,
    target_shares REAL NOT NULL, adjustment_shares REAL NOT NULL, adjustment_side TEXT NOT NULL,
    adjustment_amount REAL NOT NULL, entry_price REAL, stop_loss_price REAL, take_profit_price REAL,
    risk_amount REAL, expected_reward_amount REAL, risk_reward_ratio REAL,
    unrealized_pnl REAL NOT NULL, unrealized_pnl_pct REAL NOT NULL,
    config_json TEXT NOT NULL DEFAULT '{}', reason TEXT NOT NULL DEFAULT '[]',
    evidence TEXT NOT NULL DEFAULT '{}', trigger_conditions TEXT NOT NULL DEFAULT '[]',
    invalidation_conditions TEXT NOT NULL DEFAULT '[]', rule_version TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_position_analyses_symbol ON position_analyses(symbol, created_at DESC);
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
SELECT symbol,CASE WHEN shares>0 THEN 'LONG' ELSE 'FLAT' END,0,shares,cost_price,0,analyzed_at,current_price,
       sr_zone_analysis_id,action,action_label,shares,0,'NONE',0,stop_loss_price,take_profit_price,
       unrealized_pnl,unrealized_pnl_pct,detail_json,reason,detail_json,'[]','[]',
       'holding_sr_zone_v1_legacy',created_at FROM holding_analyses;
DROP TABLE holding_analyses;
DROP TABLE holdings;
-- +goose Down
DROP TABLE position_analyses;
DROP TABLE positions;
DROP TABLE position_transactions;
