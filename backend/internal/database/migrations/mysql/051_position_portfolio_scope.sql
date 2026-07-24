-- +goose Up
CREATE TABLE IF NOT EXISTS tenants (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS tenant_members (
    tenant_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    role VARCHAR(24) NOT NULL,
    created_at DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id),
    CONSTRAINT fk_tenant_members_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    CONSTRAINT fk_tenant_members_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS portfolios (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(128) NOT NULL,
    owner_type VARCHAR(16) NOT NULL,
    owner_id BIGINT UNSIGNED,
    created_by_user_id BIGINT UNSIGNED,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_portfolios_tenant(tenant_id, created_at DESC),
    INDEX idx_portfolios_owner(owner_type, owner_id),
    CONSTRAINT fk_portfolios_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    CONSTRAINT fk_portfolios_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO tenants(id, name, is_default) VALUES (1, 'Default Tenant', TRUE);
INSERT IGNORE INTO tenant_members(tenant_id, user_id, role)
SELECT 1, id, 'MEMBER' FROM users;
INSERT IGNORE INTO portfolios(id, tenant_id, name, owner_type, owner_id, created_by_user_id, is_default)
VALUES (1, 1, 'Legacy Shared Portfolio', 'TENANT', 1, NULL, TRUE);

ALTER TABLE position_transactions ADD COLUMN portfolio_id BIGINT UNSIGNED NOT NULL DEFAULT 1;
CREATE INDEX idx_position_transactions_portfolio_symbol_time
    ON position_transactions(portfolio_id, symbol, occurred_at, id);

ALTER TABLE positions ADD COLUMN portfolio_id BIGINT UNSIGNED NOT NULL DEFAULT 1;
ALTER TABLE positions DROP PRIMARY KEY;
ALTER TABLE positions ADD PRIMARY KEY (portfolio_id, symbol);
CREATE INDEX idx_positions_portfolio_updated ON positions(portfolio_id, updated_at DESC, symbol);

ALTER TABLE position_analyses ADD COLUMN portfolio_id BIGINT UNSIGNED NOT NULL DEFAULT 1;
CREATE INDEX idx_position_analyses_portfolio_symbol
    ON position_analyses(portfolio_id, symbol, created_at DESC);

-- +goose Down
DROP INDEX idx_position_analyses_portfolio_symbol ON position_analyses;
ALTER TABLE position_analyses DROP COLUMN portfolio_id;

DROP INDEX idx_positions_portfolio_updated ON positions;
ALTER TABLE positions DROP PRIMARY KEY;
ALTER TABLE positions ADD PRIMARY KEY (symbol);
ALTER TABLE positions DROP COLUMN portfolio_id;

DROP INDEX idx_position_transactions_portfolio_symbol_time ON position_transactions;
ALTER TABLE position_transactions DROP COLUMN portfolio_id;

DROP TABLE IF EXISTS portfolios;
DROP TABLE IF EXISTS tenant_members;
DROP TABLE IF EXISTS tenants;
