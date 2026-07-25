-- +goose Up
-- 注意：本 migration 刻意捨棄 legacy 全域持倉且不可逆（見下方 DELETE 與 issue.md I-036）。
-- NOT EXISTS 子查詢的 portfolios 包一層 derived table 強制物化，否則 MySQL 會因在 INSERT 目標表
-- portfolios 的子查詢引用自身而回 error 1093。
INSERT INTO portfolios(tenant_id, name, owner_type, owner_id, created_by_user_id, is_default)
SELECT ut.tenant_id, 'Personal Portfolio', 'USER', ut.user_id, ut.user_id, TRUE
FROM (
    SELECT user_id, MIN(tenant_id) AS tenant_id
    FROM tenant_members
    GROUP BY user_id
) ut
WHERE NOT EXISTS (
    SELECT 1
    FROM (SELECT owner_type, owner_id, is_default FROM portfolios) p
    WHERE p.owner_type='USER' AND p.owner_id=ut.user_id AND p.is_default=TRUE
);

-- 刻意、不可逆：legacy 全域持倉（portfolio_id=1）無使用者歸屬，直接清空不搬遷；
-- Down 只還原空的 Legacy portfolio row，無法還原以下被刪的持倉列。
DELETE FROM position_analyses WHERE portfolio_id=1;
DELETE FROM positions WHERE portfolio_id=1;
DELETE FROM position_transactions WHERE portfolio_id=1;

ALTER TABLE position_transactions ALTER COLUMN portfolio_id DROP DEFAULT;
ALTER TABLE positions ALTER COLUMN portfolio_id DROP DEFAULT;
ALTER TABLE position_analyses ALTER COLUMN portfolio_id DROP DEFAULT;

DELETE FROM portfolios
WHERE id=1 AND owner_type='TENANT' AND name='Legacy Shared Portfolio';

-- +goose Down
INSERT IGNORE INTO portfolios(id, tenant_id, name, owner_type, owner_id, created_by_user_id, is_default)
VALUES (1, 1, 'Legacy Shared Portfolio', 'TENANT', 1, NULL, TRUE);

ALTER TABLE position_transactions ALTER COLUMN portfolio_id SET DEFAULT 1;
ALTER TABLE positions ALTER COLUMN portfolio_id SET DEFAULT 1;
ALTER TABLE position_analyses ALTER COLUMN portfolio_id SET DEFAULT 1;
