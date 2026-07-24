-- +goose Up
CREATE TABLE IF NOT EXISTS portfolio_groups (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    name VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_portfolio_groups_tenant ON portfolio_groups(tenant_id, updated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS group_members (
    group_id BIGINT NOT NULL REFERENCES portfolio_groups(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    role VARCHAR(24) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_group_members_user ON group_members(user_id, group_id);

-- +goose Down
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS portfolio_groups;
