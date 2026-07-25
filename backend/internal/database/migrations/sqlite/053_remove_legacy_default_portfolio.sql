-- +goose Up
-- 注意：本 migration 刻意捨棄 legacy 全域持倉且不可逆（見 issue.md I-036）：下方三表 rebuild 只複製
-- portfolio_id<>1 的列，portfolio_id=1（Legacy Shared Portfolio）的持倉／交易／分析直接丟棄不搬遷，
-- Down 也無法還原。（SQLite 允許在子查詢引用 INSERT 目標表，故此處 NOT EXISTS 不需 derived table。）
INSERT INTO portfolios(tenant_id, name, owner_type, owner_id, created_by_user_id, is_default)
SELECT MIN(tm.tenant_id), 'Personal Portfolio', 'USER', u.id, u.id, 1
FROM users u
JOIN tenant_members tm ON tm.user_id=u.id
WHERE NOT EXISTS (
    SELECT 1
    FROM portfolios p
    WHERE p.owner_type='USER' AND p.owner_id=u.id AND p.is_default=1
)
GROUP BY u.id;

ALTER TABLE position_transactions RENAME TO position_transactions_legacy_scope;
ALTER TABLE positions RENAME TO positions_legacy_scope;
ALTER TABLE position_analyses RENAME TO position_analyses_legacy_scope;
DROP INDEX IF EXISTS idx_position_transactions_portfolio_symbol_time;
DROP INDEX IF EXISTS idx_positions_portfolio_updated;
DROP INDEX IF EXISTS idx_position_analyses_portfolio_symbol;

CREATE TABLE position_transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    portfolio_id INTEGER NOT NULL,
    symbol TEXT NOT NULL,
    event_type TEXT NOT NULL,
    occurred_at DATETIME NOT NULL,
    shares REAL,
    price REAL,
    fee REAL NOT NULL DEFAULT 0,
    tax REAL NOT NULL DEFAULT 0,
    target_shares REAL,
    target_avg_cost REAL,
    note TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_position_transactions_portfolio_symbol_time
    ON position_transactions(portfolio_id, symbol, occurred_at, id);

INSERT INTO position_transactions(
    id, portfolio_id, symbol, event_type, occurred_at, shares, price, fee, tax,
    target_shares, target_avg_cost, note, created_at
)
SELECT id, portfolio_id, symbol, event_type, occurred_at, shares, price, fee, tax,
       target_shares, target_avg_cost, note, created_at
FROM position_transactions_legacy_scope
WHERE portfolio_id<>1;

CREATE TABLE positions (
    portfolio_id INTEGER NOT NULL,
    symbol TEXT NOT NULL,
    shares REAL NOT NULL,
    avg_cost REAL NOT NULL,
    realized_pnl REAL NOT NULL DEFAULT 0,
    version INTEGER NOT NULL,
    last_event_id INTEGER NOT NULL REFERENCES position_transactions(id),
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portfolio_id, symbol)
);
CREATE INDEX idx_positions_portfolio_updated ON positions(portfolio_id, updated_at DESC, symbol);

INSERT INTO positions(
    portfolio_id, symbol, shares, avg_cost, realized_pnl, version, last_event_id, updated_at
)
SELECT p.portfolio_id, p.symbol, p.shares, p.avg_cost, p.realized_pnl, p.version, p.last_event_id, p.updated_at
FROM positions_legacy_scope p
JOIN position_transactions tx ON tx.id=p.last_event_id
WHERE p.portfolio_id<>1;

CREATE TABLE position_analyses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    portfolio_id INTEGER NOT NULL,
    symbol TEXT NOT NULL,
    position_state TEXT NOT NULL,
    position_version INTEGER NOT NULL,
    shares REAL NOT NULL,
    avg_cost REAL NOT NULL,
    realized_pnl REAL NOT NULL,
    analyzed_at DATETIME NOT NULL,
    current_price REAL NOT NULL,
    sr_zone_analysis_id INTEGER,
    action TEXT NOT NULL,
    action_label TEXT NOT NULL,
    target_shares REAL NOT NULL,
    adjustment_shares REAL NOT NULL,
    adjustment_side TEXT NOT NULL,
    adjustment_amount REAL NOT NULL,
    entry_price REAL,
    stop_loss_price REAL,
    take_profit_price REAL,
    risk_amount REAL,
    expected_reward_amount REAL,
    risk_reward_ratio REAL,
    unrealized_pnl REAL NOT NULL,
    unrealized_pnl_pct REAL NOT NULL,
    config_json TEXT NOT NULL,
    reason TEXT NOT NULL,
    evidence TEXT NOT NULL,
    trigger_conditions TEXT NOT NULL,
    invalidation_conditions TEXT NOT NULL,
    rule_version TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_position_analyses_portfolio_symbol
    ON position_analyses(portfolio_id, symbol, created_at DESC);

INSERT INTO position_analyses(
    id, portfolio_id, symbol, position_state, position_version, shares, avg_cost,
    realized_pnl, analyzed_at, current_price, sr_zone_analysis_id, action, action_label,
    target_shares, adjustment_shares, adjustment_side, adjustment_amount, entry_price,
    stop_loss_price, take_profit_price, risk_amount, expected_reward_amount, risk_reward_ratio,
    unrealized_pnl, unrealized_pnl_pct, config_json, reason, evidence, trigger_conditions,
    invalidation_conditions, rule_version, created_at
)
SELECT id, portfolio_id, symbol, position_state, position_version, shares, avg_cost,
       realized_pnl, analyzed_at, current_price, sr_zone_analysis_id, action, action_label,
       target_shares, adjustment_shares, adjustment_side, adjustment_amount, entry_price,
       stop_loss_price, take_profit_price, risk_amount, expected_reward_amount, risk_reward_ratio,
       unrealized_pnl, unrealized_pnl_pct, config_json, reason, evidence, trigger_conditions,
       invalidation_conditions, rule_version, created_at
FROM position_analyses_legacy_scope
WHERE portfolio_id<>1;

DROP TABLE position_analyses_legacy_scope;
DROP TABLE positions_legacy_scope;
DROP TABLE position_transactions_legacy_scope;

DELETE FROM portfolios
WHERE id=1 AND owner_type='TENANT' AND name='Legacy Shared Portfolio';

-- +goose Down
INSERT OR IGNORE INTO portfolios(id, tenant_id, name, owner_type, owner_id, created_by_user_id, is_default)
VALUES (1, 1, 'Legacy Shared Portfolio', 'TENANT', 1, NULL, 1);

ALTER TABLE position_transactions RENAME TO position_transactions_explicit_scope;
ALTER TABLE positions RENAME TO positions_explicit_scope;
ALTER TABLE position_analyses RENAME TO position_analyses_explicit_scope;
DROP INDEX IF EXISTS idx_position_transactions_portfolio_symbol_time;
DROP INDEX IF EXISTS idx_positions_portfolio_updated;
DROP INDEX IF EXISTS idx_position_analyses_portfolio_symbol;

CREATE TABLE position_transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    portfolio_id INTEGER NOT NULL DEFAULT 1,
    symbol TEXT NOT NULL,
    event_type TEXT NOT NULL,
    occurred_at DATETIME NOT NULL,
    shares REAL,
    price REAL,
    fee REAL NOT NULL DEFAULT 0,
    tax REAL NOT NULL DEFAULT 0,
    target_shares REAL,
    target_avg_cost REAL,
    note TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_position_transactions_portfolio_symbol_time
    ON position_transactions(portfolio_id, symbol, occurred_at, id);

INSERT INTO position_transactions(
    id, portfolio_id, symbol, event_type, occurred_at, shares, price, fee, tax,
    target_shares, target_avg_cost, note, created_at
)
SELECT id, portfolio_id, symbol, event_type, occurred_at, shares, price, fee, tax,
       target_shares, target_avg_cost, note, created_at
FROM position_transactions_explicit_scope;

CREATE TABLE positions (
    portfolio_id INTEGER NOT NULL DEFAULT 1,
    symbol TEXT NOT NULL,
    shares REAL NOT NULL,
    avg_cost REAL NOT NULL,
    realized_pnl REAL NOT NULL DEFAULT 0,
    version INTEGER NOT NULL,
    last_event_id INTEGER NOT NULL REFERENCES position_transactions(id),
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portfolio_id, symbol)
);
CREATE INDEX idx_positions_portfolio_updated ON positions(portfolio_id, updated_at DESC, symbol);

INSERT INTO positions(
    portfolio_id, symbol, shares, avg_cost, realized_pnl, version, last_event_id, updated_at
)
SELECT portfolio_id, symbol, shares, avg_cost, realized_pnl, version, last_event_id, updated_at
FROM positions_explicit_scope;

CREATE TABLE position_analyses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    portfolio_id INTEGER NOT NULL DEFAULT 1,
    symbol TEXT NOT NULL,
    position_state TEXT NOT NULL,
    position_version INTEGER NOT NULL,
    shares REAL NOT NULL,
    avg_cost REAL NOT NULL,
    realized_pnl REAL NOT NULL,
    analyzed_at DATETIME NOT NULL,
    current_price REAL NOT NULL,
    sr_zone_analysis_id INTEGER,
    action TEXT NOT NULL,
    action_label TEXT NOT NULL,
    target_shares REAL NOT NULL,
    adjustment_shares REAL NOT NULL,
    adjustment_side TEXT NOT NULL,
    adjustment_amount REAL NOT NULL,
    entry_price REAL,
    stop_loss_price REAL,
    take_profit_price REAL,
    risk_amount REAL,
    expected_reward_amount REAL,
    risk_reward_ratio REAL,
    unrealized_pnl REAL NOT NULL,
    unrealized_pnl_pct REAL NOT NULL,
    config_json TEXT NOT NULL,
    reason TEXT NOT NULL,
    evidence TEXT NOT NULL,
    trigger_conditions TEXT NOT NULL,
    invalidation_conditions TEXT NOT NULL,
    rule_version TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_position_analyses_portfolio_symbol
    ON position_analyses(portfolio_id, symbol, created_at DESC);

INSERT INTO position_analyses(
    id, portfolio_id, symbol, position_state, position_version, shares, avg_cost,
    realized_pnl, analyzed_at, current_price, sr_zone_analysis_id, action, action_label,
    target_shares, adjustment_shares, adjustment_side, adjustment_amount, entry_price,
    stop_loss_price, take_profit_price, risk_amount, expected_reward_amount, risk_reward_ratio,
    unrealized_pnl, unrealized_pnl_pct, config_json, reason, evidence, trigger_conditions,
    invalidation_conditions, rule_version, created_at
)
SELECT id, portfolio_id, symbol, position_state, position_version, shares, avg_cost,
       realized_pnl, analyzed_at, current_price, sr_zone_analysis_id, action, action_label,
       target_shares, adjustment_shares, adjustment_side, adjustment_amount, entry_price,
       stop_loss_price, take_profit_price, risk_amount, expected_reward_amount, risk_reward_ratio,
       unrealized_pnl, unrealized_pnl_pct, config_json, reason, evidence, trigger_conditions,
       invalidation_conditions, rule_version, created_at
FROM position_analyses_explicit_scope;

DROP TABLE position_analyses_explicit_scope;
DROP TABLE positions_explicit_scope;
DROP TABLE position_transactions_explicit_scope;
