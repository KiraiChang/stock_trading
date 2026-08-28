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
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGroupAccessDenied
	}
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
//
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
	// 目標 user 必須「已是」group tenant 的成員才能加入 group。不自動補 tenant membership，
	// 避免 group admin 靜默把任意 user_id 拉進 tenant 而取得 TENANT portfolio 寫入權（提權副作用）。
	// 現行單一 default tenant 下每個註冊 user 都已是 default tenant 成員、group 也都在該 tenant，
	// 故此檢查不影響一般流程；只有跨租戶（把別 tenant 的 user 加進本 group）才會被明確拒絕，
	// 取代原本 CanAccess 對非 tenant 成員的 silent lockout。
	tenantID, err := groupTenantTx(ctx, tx, groupID)
	if err != nil {
		return err
	}
	isMember, err := isTenantMemberTx(ctx, tx, tenantID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrGroupAccessDenied
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

// isTenantMemberTx 回傳 user 是否已是該 tenant 的成員。
func isTenantMemberTx(ctx context.Context, tx *sqlx.Tx, tenantID uint64, userID uint64) (bool, error) {
	var count int
	err := tx.GetContext(ctx, &count, tx.Rebind(`
		SELECT COUNT(1) FROM tenant_members WHERE tenant_id=? AND user_id=?
	`), tenantID, userID)
	return count > 0, err
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

// firstTenantForUser 回傳 user 最先加入的 tenant id（store 內共用）。查無 membership 時回傳
// 原始 sql.ErrNoRows，由各呼叫端映射成自己的 access-denied error（group / portfolio）。
func firstTenantForUser(ctx context.Context, db *sqlx.DB, userID uint64) (uint64, error) {
	var id uint64
	err := db.GetContext(ctx, &id, db.Rebind(`
		SELECT tenant_id FROM tenant_members WHERE user_id=? ORDER BY tenant_id LIMIT 1
	`), userID)
	return id, err
}
