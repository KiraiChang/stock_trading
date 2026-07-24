-- +goose Up
CREATE TABLE IF NOT EXISTS tenants (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tenant_members (
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    role VARCHAR(24) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id)
);

CREATE TABLE IF NOT EXISTS portfolios (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    name VARCHAR(128) NOT NULL,
    owner_type VARCHAR(16) NOT NULL,
    owner_id BIGINT,
    created_by_user_id BIGINT REFERENCES users(id),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_portfolios_tenant ON portfolios(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_portfolios_owner ON portfolios(owner_type, owner_id);

INSERT INTO tenants(id, name, is_default)
VALUES (1, 'Default Tenant', TRUE)
ON CONFLICT (id) DO NOTHING;
SELECT setval(pg_get_serial_sequence('tenants', 'id'), COALESCE((SELECT MAX(id) FROM tenants), 1));

INSERT INTO tenant_members(tenant_id, user_id, role)
SELECT 1, id, 'MEMBER' FROM users
ON CONFLICT (tenant_id, user_id) DO NOTHING;

INSERT INTO portfolios(id, tenant_id, name, owner_type, owner_id, created_by_user_id, is_default)
VALUES (1, 1, 'Legacy Shared Portfolio', 'TENANT', 1, NULL, TRUE)
ON CONFLICT (id) DO NOTHING;
SELECT setval(pg_get_serial_sequence('portfolios', 'id'), COALESCE((SELECT MAX(id) FROM portfolios), 1));

ALTER TABLE position_transactions
    ADD COLUMN portfolio_id BIGINT NOT NULL DEFAULT 1 REFERENCES portfolios(id);
CREATE INDEX idx_position_transactions_portfolio_symbol_time
    ON position_transactions(portfolio_id, symbol, occurred_at, id);

ALTER TABLE positions
    ADD COLUMN portfolio_id BIGINT NOT NULL DEFAULT 1 REFERENCES portfolios(id);
ALTER TABLE positions DROP CONSTRAINT positions_pkey;
ALTER TABLE positions ADD PRIMARY KEY (portfolio_id, symbol);
CREATE INDEX idx_positions_portfolio_updated ON positions(portfolio_id, updated_at DESC, symbol);

ALTER TABLE position_analyses
    ADD COLUMN portfolio_id BIGINT NOT NULL DEFAULT 1 REFERENCES portfolios(id);
CREATE INDEX idx_position_analyses_portfolio_symbol
    ON position_analyses(portfolio_id, symbol, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_position_analyses_portfolio_symbol;
ALTER TABLE position_analyses DROP COLUMN IF EXISTS portfolio_id;

DROP INDEX IF EXISTS idx_positions_portfolio_updated;
ALTER TABLE positions DROP CONSTRAINT positions_pkey;
ALTER TABLE positions ADD PRIMARY KEY (symbol);
ALTER TABLE positions DROP COLUMN IF EXISTS portfolio_id;

DROP INDEX IF EXISTS idx_position_transactions_portfolio_symbol_time;
ALTER TABLE position_transactions DROP COLUMN IF EXISTS portfolio_id;

DROP TABLE IF EXISTS portfolios;
DROP TABLE IF EXISTS tenant_members;
DROP TABLE IF EXISTS tenants;
