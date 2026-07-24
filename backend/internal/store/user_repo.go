package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

type User struct {
	ID           uint64    `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Status       string    `db:"status"`
	CreatedAt    time.Time `db:"created_at"`
}

type UserRepo interface {
	Create(ctx context.Context, email, passwordHash string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context) ([]User, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
}

type userRepo struct {
	db     *sqlx.DB
	driver string
}

func NewUserRepo(db *sqlx.DB) UserRepo {
	return &userRepo{db: db, driver: db.DriverName()}
}

func (r *userRepo) Create(ctx context.Context, email, passwordHash string) (*User, error) {
	// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
	if r.driver == "pgx" {
		var id uint64
		err := r.db.QueryRowContext(ctx,
			`INSERT INTO users (email, password_hash, status) VALUES ($1, $2, 'inactive') RETURNING id`,
			email, passwordHash,
		).Scan(&id)
		if err != nil {
			return nil, err
		}
		if err := r.addDefaultTenantMembership(ctx, id); err != nil {
			return nil, err
		}
		return &User{ID: id, Email: email, Status: "inactive", CreatedAt: time.Now()}, nil
	}

	sql := r.db.Rebind(`INSERT INTO users (email, password_hash, status) VALUES (?, ?, 'inactive')`)
	res, err := r.db.ExecContext(ctx, sql, email, passwordHash)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	userID := uint64(id)
	if err := r.addDefaultTenantMembership(ctx, userID); err != nil {
		return nil, err
	}
	return &User{ID: userID, Email: email, Status: "inactive", CreatedAt: time.Now()}, nil
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	sql := r.db.Rebind(`SELECT id, email, password_hash, status, created_at FROM users WHERE email=? LIMIT 1`)
	if err := r.db.GetContext(ctx, &u, sql, email); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) List(ctx context.Context) ([]User, error) {
	var users []User
	err := r.db.SelectContext(ctx, &users,
		`SELECT id, email, password_hash, status, created_at FROM users ORDER BY created_at DESC`)
	return users, err
}

func (r *userRepo) UpdateStatus(ctx context.Context, id uint64, status string) error {
	sql := r.db.Rebind(`UPDATE users SET status=? WHERE id=?`)
	_, err := r.db.ExecContext(ctx, sql, status, id)
	return err
}

func (r *userRepo) addDefaultTenantMembership(ctx context.Context, userID uint64) error {
	if r.driver == "pgx" {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO tenant_members(tenant_id,user_id,role)
			SELECT id,$1,'MEMBER' FROM tenants WHERE is_default=TRUE ORDER BY id LIMIT 1
			ON CONFLICT (tenant_id,user_id) DO NOTHING
		`, userID)
		return err
	}
	if r.driver == "mysql" {
		_, err := r.db.ExecContext(ctx, `
			INSERT IGNORE INTO tenant_members(tenant_id,user_id,role)
			SELECT id,?,'MEMBER' FROM tenants WHERE is_default=TRUE ORDER BY id LIMIT 1
		`, userID)
		return err
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT OR IGNORE INTO tenant_members(tenant_id,user_id,role)
		SELECT id,?,'MEMBER' FROM tenants WHERE is_default=1 ORDER BY id LIMIT 1
	`), userID)
	return err
}
