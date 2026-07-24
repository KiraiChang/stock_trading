-- +goose Up
CREATE TABLE IF NOT EXISTS portfolio_groups (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(128) NOT NULL,
    created_at DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_portfolio_groups_tenant(tenant_id, updated_at DESC, id DESC),
    CONSTRAINT fk_groups_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS group_members (
    group_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    role VARCHAR(24) NOT NULL,
    created_at DATETIME(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id),
    INDEX idx_group_members_user(user_id, group_id),
    CONSTRAINT fk_group_members_group FOREIGN KEY (group_id) REFERENCES portfolio_groups(id),
    CONSTRAINT fk_group_members_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS portfolio_groups;
