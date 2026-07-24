package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
)

const (
	GroupRoleOwner  = "OWNER"
	GroupRoleAdmin  = "ADMIN"
	GroupRoleMember = "MEMBER"
	GroupRoleViewer = "VIEWER"
)

var ErrGroupAccessDenied = errors.New("group access denied")

type GroupRepo interface {
	ListForUser(ctx context.Context, userID uint64) ([]Group, error)
	Create(ctx context.Context, userID uint64, name string) (*Group, error)
	AddMember(ctx context.Context, actorUserID uint64, groupID uint64, userID uint64, role string) error
}

type groupRepo struct {
	db     *sqlx.DB
	driver string
}

func NewGroupRepo(db *sqlx.DB) GroupRepo {
	return &groupRepo{db: db, driver: db.DriverName()}
}

func (r *groupRepo) ListForUser(ctx context.Context, userID uint64) ([]Group, error) {
	var rows []Group
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT g.id,g.tenant_id,g.name,g.created_at,g.updated_at
		FROM portfolio_groups g
		JOIN group_members gm ON gm.group_id=g.id
		WHERE gm.user_id=?
		ORDER BY g.updated_at DESC,g.id DESC
	`), userID)
	return rows, err
}

func (r *groupRepo) Create(ctx context.Context, userID uint64, name string) (*Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "New Group"
	}
	tenantID, err := firstTenantForUser(ctx, r.db, userID)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id uint64
	if r.driver == "pgx" {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO portfolio_groups(tenant_id,name) VALUES($1,$2) RETURNING id
		`, tenantID, name).Scan(&id)
	} else {
		res, execErr := tx.ExecContext(ctx, tx.Rebind(`INSERT INTO portfolio_groups(tenant_id,name) VALUES(?,?)`), tenantID, name)
		if execErr != nil {
			return nil, execErr
		}
		lastID, lastErr := res.LastInsertId()
		if lastErr != nil {
			return nil, lastErr
		}
		id = uint64(lastID)
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`
		INSERT INTO group_members(group_id,user_id,role) VALUES(?,?,?)
	`), id, userID, GroupRoleOwner); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.get(ctx, id)
}

// AddMember 新增或更新成員角色，並套用角色保護，避免管理權被濫用：
//   - actor 必須是 OWNER / ADMIN 才能管理成員。
//   - actor 不得修改自己的角色（擋 ADMIN 自我提權為 OWNER、擋 OWNER 自我降級鎖出）。
//   - 只有 OWNER 能授予 OWNER 角色，或異動一個現任 OWNER 的角色；ADMIN 不得碰 OWNER。
//   - 不得把最後一名 OWNER 降級，避免 group 變成無 OWNER。
// check 與 upsert 放在同一個交易內，避免併發下最後一名 OWNER 判斷產生 race。
func (r *groupRepo) AddMember(ctx context.Context, actorUserID uint64, groupID uint64, userID uint64, role string) error {
	role = normalizeGroupRole(role)
	if role == "" {
		return ErrGroupAccessDenied
	}
	if actorUserID == userID {
		return ErrGroupAccessDenied
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	actorRole, err := groupRoleTx(ctx, tx, groupID, actorUserID)
	if err != nil {
		return err
	}
	if actorRole != GroupRoleOwner && actorRole != GroupRoleAdmin {
		return ErrGroupAccessDenied
	}
	targetRole, err := groupRoleTx(ctx, tx, groupID, userID)
	if err != nil {
		return err
	}
	// 授予 OWNER，或異動一個現任 OWNER，都只有 OWNER 能做。
	if (role == GroupRoleOwner || targetRole == GroupRoleOwner) && actorRole != GroupRoleOwner {
		return ErrGroupAccessDenied
	}
	// 把現任 OWNER 降成非 OWNER 前，確認不是最後一名 OWNER。
	if targetRole == GroupRoleOwner && role != GroupRoleOwner {
		owners, err := groupOwnerCountTx(ctx, tx, groupID)
		if err != nil {
			return err
		}
		if owners <= 1 {
			return ErrGroupAccessDenied
		}
	}
	// group membership 必須同時保證 group tenant 的 membership，否則
	// portfolio_repo.CanAccess 對 GROUP portfolio 的 tenant join 會把該成員靜默鎖死。
	tenantID, err := groupTenantTx(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if err := ensureTenantMemberTx(ctx, tx, r.driver, tenantID, userID); err != nil {
		return err
	}
	if err := upsertGroupMemberTx(ctx, tx, r.driver, groupID, userID, role); err != nil {
		return err
	}
	return tx.Commit()
}

// groupTenantTx 回傳 group 所屬的 tenant id。
func groupTenantTx(ctx context.Context, tx *sqlx.Tx, groupID uint64) (uint64, error) {
	var tenantID uint64
	err := tx.GetContext(ctx, &tenantID, tx.Rebind(`SELECT tenant_id FROM portfolio_groups WHERE id=?`), groupID)
	return tenantID, err
}

// ensureTenantMemberTx 若 user 尚非該 tenant 成員，補上 MEMBER；已是成員則不動其角色。
func ensureTenantMemberTx(ctx context.Context, tx *sqlx.Tx, driver string, tenantID uint64, userID uint64) error {
	if driver == "pgx" {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO tenant_members(tenant_id,user_id,role) VALUES($1,$2,'MEMBER')
			ON CONFLICT(tenant_id,user_id) DO NOTHING
		`, tenantID, userID)
		return err
	}
	if driver == "mysql" {
		_, err := tx.ExecContext(ctx, `
			INSERT IGNORE INTO tenant_members(tenant_id,user_id,role) VALUES(?,?,'MEMBER')
		`, tenantID, userID)
		return err
	}
	_, err := tx.ExecContext(ctx, tx.Rebind(`
		INSERT OR IGNORE INTO tenant_members(tenant_id,user_id,role) VALUES(?,?,'MEMBER')
	`), tenantID, userID)
	return err
}

// groupRoleTx 回傳 user 在 group 內的角色；非成員回傳空字串。
func groupRoleTx(ctx context.Context, tx *sqlx.Tx, groupID uint64, userID uint64) (string, error) {
	var role string
	err := tx.GetContext(ctx, &role, tx.Rebind(`
		SELECT role FROM group_members WHERE group_id=? AND user_id=?
	`), groupID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}

func groupOwnerCountTx(ctx context.Context, tx *sqlx.Tx, groupID uint64) (int, error) {
	var count int
	err := tx.GetContext(ctx, &count, tx.Rebind(`
		SELECT COUNT(1) FROM group_members WHERE group_id=? AND role='OWNER'
	`), groupID)
	return count, err
}

func upsertGroupMemberTx(ctx context.Context, tx *sqlx.Tx, driver string, groupID uint64, userID uint64, role string) error {
	if driver == "pgx" {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO group_members(group_id,user_id,role) VALUES($1,$2,$3)
			ON CONFLICT(group_id,user_id) DO UPDATE SET role=EXCLUDED.role
		`, groupID, userID, role)
		return err
	}
	if driver == "mysql" {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO group_members(group_id,user_id,role) VALUES(?,?,?)
			ON DUPLICATE KEY UPDATE role=VALUES(role)
		`, groupID, userID, role)
		return err
	}
	_, err := tx.ExecContext(ctx, tx.Rebind(`
		INSERT INTO group_members(group_id,user_id,role) VALUES(?,?,?)
		ON CONFLICT(group_id,user_id) DO UPDATE SET role=excluded.role
	`), groupID, userID, role)
	return err
}

func (r *groupRepo) get(ctx context.Context, id uint64) (*Group, error) {
	var row Group
	err := r.db.GetContext(ctx, &row, r.db.Rebind(`
		SELECT id,tenant_id,name,created_at,updated_at FROM portfolio_groups WHERE id=?
	`), id)
	return &row, err
}

func normalizeGroupRole(role string) string {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case GroupRoleOwner:
		return GroupRoleOwner
	case GroupRoleAdmin:
		return GroupRoleAdmin
	case GroupRoleMember:
		return GroupRoleMember
	case GroupRoleViewer:
		return GroupRoleViewer
	default:
		return ""
	}
}

func firstTenantForUser(ctx context.Context, db *sqlx.DB, userID uint64) (uint64, error) {
	var id uint64
	err := db.GetContext(ctx, &id, db.Rebind(`
		SELECT tenant_id FROM tenant_members WHERE user_id=? ORDER BY tenant_id LIMIT 1
	`), userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrGroupAccessDenied
	}
	return id, err
}
