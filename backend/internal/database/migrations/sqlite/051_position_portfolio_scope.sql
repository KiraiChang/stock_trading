-- +goose Up
CREATE TABLE IF NOT EXISTS tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tenant_members (
    tenant_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    role TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id)
);

CREATE TABLE IF NOT EXISTS portfolios (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    owner_type TEXT NOT NULL,
    owner_id INTEGER,
    created_by_user_id INTEGER,
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_portfolios_tenant ON portfolios(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_portfolios_owner ON portfolios(owner_type, owner_id);

INSERT OR IGNORE INTO tenants(id, name, is_default) VALUES (1, 'Default Tenant', 1);
INSERT OR IGNORE INTO tenant_members(tenant_id, user_id, role)
SELECT 1, id, 'MEMBER' FROM users;
INSERT OR IGNORE INTO portfolios(id, tenant_id, name, owner_type, owner_id, created_by_user_id, is_default)
VALUES (1, 1, 'Legacy Shared Portfolio', 'TENANT', 1, NULL, 1);

ALTER TABLE position_transactions ADD COLUMN portfolio_id INTEGER NOT NULL DEFAULT 1;
CREATE INDEX IF NOT EXISTS idx_position_transactions_portfolio_symbol_time
    ON position_transactions(portfolio_id, symbol, occurred_at, id);

ALTER TABLE position_analyses ADD COLUMN portfolio_id INTEGER NOT NULL DEFAULT 1;
CREATE INDEX IF NOT EXISTS idx_position_analyses_portfolio_symbol
    ON position_analyses(portfolio_id, symbol, created_at DESC);

ALTER TABLE positions RENAME TO positions_old;
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
INSERT INTO positions(portfolio_id, symbol, shares, avg_cost, realized_pnl, version, last_event_id, updated_at)
SELECT 1, symbol, shares, avg_cost, realized_pnl, version, last_event_id, updated_at
FROM positions_old;
DROP TABLE positions_old;
CREATE INDEX IF NOT EXISTS idx_positions_portfolio_updated ON positions(portfolio_id, updated_at DESC, symbol);

-- +goose Down
DROP INDEX IF EXISTS idx_positions_portfolio_updated;
ALTER TABLE positions RENAME TO positions_scoped;
CREATE TABLE positions (
    symbol TEXT PRIMARY KEY,
    shares REAL NOT NULL,
    avg_cost REAL NOT NULL,
    realized_pnl REAL NOT NULL DEFAULT 0,
    version INTEGER NOT NULL,
    last_event_id INTEGER NOT NULL REFERENCES position_transactions(id),
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO positions(symbol, shares, avg_cost, realized_pnl, version, last_event_id, updated_at)
SELECT symbol, shares, avg_cost, realized_pnl, version, last_event_id, updated_at
FROM positions_scoped
WHERE portfolio_id = 1;
DROP TABLE positions_scoped;

DROP INDEX IF EXISTS idx_position_analyses_portfolio_symbol;
ALTER TABLE position_analyses DROP COLUMN portfolio_id;

DROP INDEX IF EXISTS idx_position_transactions_portfolio_symbol_time;
ALTER TABLE position_transactions DROP COLUMN portfolio_id;

DROP TABLE IF EXISTS portfolios;
DROP TABLE IF EXISTS tenant_members;
DROP TABLE IF EXISTS tenants;
