-- +goose Up
-- 注意：本 migration 刻意捨棄 legacy 全域持倉且不可逆（見下方 DELETE 與 issue.md I-036）。
WITH user_tenants AS (
    SELECT user_id, MIN(tenant_id) AS tenant_id
    FROM tenant_members
    GROUP BY user_id
)
INSERT INTO portfolios(tenant_id, name, owner_type, owner_id, created_by_user_id, is_default)
SELECT ut.tenant_id, 'Personal Portfolio', 'USER', ut.user_id, ut.user_id, TRUE
FROM user_tenants ut
WHERE NOT EXISTS (
    SELECT 1
    FROM portfolios p
    WHERE p.owner_type='USER' AND p.owner_id=ut.user_id AND p.is_default=TRUE
);
SELECT setval(pg_get_serial_sequence('portfolios', 'id'), COALESCE((SELECT MAX(id) FROM portfolios), 1));

-- 刻意、不可逆：legacy 全域持倉（portfolio_id=1）無使用者歸屬，直接清空不搬遷；
-- Down 只還原空的 Legacy portfolio row，無法還原被刪的持倉列。
DELETE FROM position_analyses WHERE portfolio_id=1;
DELETE FROM positions WHERE portfolio_id=1;
DELETE FROM position_transactions WHERE portfolio_id=1;

ALTER TABLE position_transactions ALTER COLUMN portfolio_id DROP DEFAULT;
ALTER TABLE positions ALTER COLUMN portfolio_id DROP DEFAULT;
ALTER TABLE position_analyses ALTER COLUMN portfolio_id DROP DEFAULT;

DELETE FROM portfolios
WHERE id=1 AND owner_type='TENANT' AND name='Legacy Shared Portfolio';

-- +goose Down
INSERT INTO portfolios(id, tenant_id, name, owner_type, owner_id, created_by_user_id, is_default)
VALUES (1, 1, 'Legacy Shared Portfolio', 'TENANT', 1, NULL, TRUE)
ON CONFLICT (id) DO NOTHING;
SELECT setval(pg_get_serial_sequence('portfolios', 'id'), COALESCE((SELECT MAX(id) FROM portfolios), 1));

ALTER TABLE position_transactions ALTER COLUMN portfolio_id SET DEFAULT 1;
ALTER TABLE positions ALTER COLUMN portfolio_id SET DEFAULT 1;
ALTER TABLE position_analyses ALTER COLUMN portfolio_id SET DEFAULT 1;
