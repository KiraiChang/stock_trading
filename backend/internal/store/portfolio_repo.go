package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
)

const (
	PortfolioOwnerUser   = "USER"
	PortfolioOwnerTenant = "TENANT"
	PortfolioOwnerGroup  = "GROUP"
)

var ErrPortfolioAccessDenied = errors.New("portfolio access denied")

type PortfolioRepo interface {
	ListForUser(ctx context.Context, userID uint64) ([]Portfolio, error)
	CreateForUser(ctx context.Context, userID uint64, name string) (*Portfolio, error)
	CreateForGroup(ctx context.Context, userID uint64, groupID uint64, name string) (*Portfolio, error)
	CanAccess(ctx context.Context, userID uint64, portfolioID uint64, write bool) (bool, error)
}

type portfolioRepo struct {
	db     *sqlx.DB
	driver string
}

func NewPortfolioRepo(db *sqlx.DB) PortfolioRepo {
	return &portfolioRepo{db: db, driver: db.DriverName()}
}

func (r *portfolioRepo) ListForUser(ctx context.Context, userID uint64) ([]Portfolio, error) {
	var rows []Portfolio
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT DISTINCT p.id,p.tenant_id,p.name,p.owner_type,p.owner_id,p.created_by_user_id,
		       p.is_default,
		       CASE
		         WHEN p.owner_type='GROUP' THEN CASE WHEN gm.role IN ('OWNER','ADMIN') THEN TRUE ELSE FALSE END
		         ELSE TRUE
		       END AS can_write,
		       p.created_at,p.updated_at
		FROM portfolios p
		JOIN tenant_members tm ON tm.tenant_id=p.tenant_id
		LEFT JOIN group_members gm ON gm.group_id=p.owner_id AND p.owner_type='GROUP' AND gm.user_id=tm.user_id
		WHERE tm.user_id=?
		  AND (
		    p.owner_type='TENANT'
		    OR (p.owner_type='USER' AND p.owner_id=?)
		    OR (p.owner_type='GROUP' AND gm.user_id IS NOT NULL)
		  )
		ORDER BY p.is_default DESC,p.created_at DESC,p.id DESC
	`), userID, userID)
	return rows, err
}

func (r *portfolioRepo) CreateForUser(ctx context.Context, userID uint64, name string) (*Portfolio, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Personal Portfolio"
	}
	tenantID, err := r.defaultTenantForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	ownerID := NewNullInt64(int64(userID))
	if r.driver == "pgx" {
		var id uint64
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO portfolios(tenant_id,name,owner_type,owner_id,created_by_user_id,is_default)
			VALUES($1,$2,$3,$4,$5,FALSE) RETURNING id
		`, tenantID, name, PortfolioOwnerUser, nullInt64Value(ownerID), userID).Scan(&id)
		if err != nil {
			return nil, err
		}
		return r.get(ctx, id, true)
	}
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT INTO portfolios(tenant_id,name,owner_type,owner_id,created_by_user_id,is_default)
		VALUES(?,?,?,?,?,?)
	`), tenantID, name, PortfolioOwnerUser, nullInt64Value(ownerID), userID, false)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.get(ctx, uint64(id), true)
}

func (r *portfolioRepo) CreateForGroup(ctx context.Context, userID uint64, groupID uint64, name string) (*Portfolio, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Group Portfolio"
	}
	var tenantID uint64
	err := r.db.GetContext(ctx, &tenantID, r.db.Rebind(`
		SELECT g.tenant_id
		FROM portfolio_groups g
		JOIN group_members gm ON gm.group_id=g.id
		WHERE g.id=? AND gm.user_id=? AND gm.role IN ('OWNER','ADMIN')
		LIMIT 1
	`), groupID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPortfolioAccessDenied
	}
	if err != nil {
		return nil, err
	}
	ownerID := NewNullInt64(int64(groupID))
	if r.driver == "pgx" {
		var id uint64
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO portfolios(tenant_id,name,owner_type,owner_id,created_by_user_id,is_default)
			VALUES($1,$2,$3,$4,$5,FALSE) RETURNING id
		`, tenantID, name, PortfolioOwnerGroup, nullInt64Value(ownerID), userID).Scan(&id)
		if err != nil {
			return nil, err
		}
		return r.get(ctx, id, true)
	}
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT INTO portfolios(tenant_id,name,owner_type,owner_id,created_by_user_id,is_default)
		VALUES(?,?,?,?,?,?)
	`), tenantID, name, PortfolioOwnerGroup, nullInt64Value(ownerID), userID, false)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.get(ctx, uint64(id), true)
}

func (r *portfolioRepo) CanAccess(ctx context.Context, userID uint64, portfolioID uint64, write bool) (bool, error) {
	portfolioID = normalizePortfolioID(portfolioID)
	var count int
	roleFilter := ""
	if write {
		roleFilter = " AND (p.owner_type!='GROUP' OR gm.role IN ('OWNER','ADMIN'))"
	}
	err := r.db.GetContext(ctx, &count, r.db.Rebind(`
		SELECT COUNT(1)
		FROM portfolios p
		JOIN tenant_members tm ON tm.tenant_id=p.tenant_id
		LEFT JOIN group_members gm ON gm.group_id=p.owner_id AND p.owner_type='GROUP' AND gm.user_id=tm.user_id
		WHERE p.id=? AND tm.user_id=?
		  AND (
		    p.owner_type='TENANT'
		    OR (p.owner_type='USER' AND p.owner_id=?)
		    OR (p.owner_type='GROUP' AND gm.user_id IS NOT NULL`+roleFilter+`)
		  )
	`), portfolioID, userID, userID)
	return count > 0, err
}

func (r *portfolioRepo) defaultTenantForUser(ctx context.Context, userID uint64) (uint64, error) {
	var id uint64
	err := r.db.GetContext(ctx, &id, r.db.Rebind(`
		SELECT tenant_id FROM tenant_members WHERE user_id=? ORDER BY tenant_id LIMIT 1
	`), userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrPortfolioAccessDenied
	}
	return id, err
}

func (r *portfolioRepo) get(ctx context.Context, id uint64, canWrite bool) (*Portfolio, error) {
	var row Portfolio
	err := r.db.GetContext(ctx, &row, r.db.Rebind(`
		SELECT id,tenant_id,name,owner_type,owner_id,created_by_user_id,is_default,created_at,updated_at
		FROM portfolios WHERE id=?
	`), id)
	row.CanWrite = canWrite
	return &row, err
}
