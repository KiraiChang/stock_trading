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
	CreatedAt    time.Time `db:"created_at"`
}

type UserRepo interface {
	Create(ctx context.Context, email, passwordHash string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
}

type userRepo struct {
	db *sqlx.DB
}

func NewUserRepo(db *sqlx.DB) UserRepo {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, email, passwordHash string) (*User, error) {
	sql := r.db.Rebind(`INSERT INTO users (email, password_hash) VALUES (?, ?)`)
	res, err := r.db.ExecContext(ctx, sql, email, passwordHash)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: uint64(id), Email: email, CreatedAt: time.Now()}, nil
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	sql := r.db.Rebind(`SELECT id, email, password_hash, created_at FROM users WHERE email=? LIMIT 1`)
	if err := r.db.GetContext(ctx, &u, sql, email); err != nil {
		return nil, err
	}
	return &u, nil
}
