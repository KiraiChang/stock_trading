package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type WatchlistRepo interface {
	GetAll(ctx context.Context) ([]WatchlistItem, error)
	Add(ctx context.Context, symbol, name, sector string) error
	Remove(ctx context.Context, symbol string) error
	Symbols(ctx context.Context) ([]string, error)
}

type watchlistRepo struct {
	db *sqlx.DB
}

func NewWatchlistRepo(db *sqlx.DB) WatchlistRepo {
	return &watchlistRepo{db: db}
}

func (r *watchlistRepo) GetAll(ctx context.Context) ([]WatchlistItem, error) {
	var rows []WatchlistItem
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, symbol, name, sector FROM watchlists ORDER BY added_at ASC
	`)
	return rows, err
}

func (r *watchlistRepo) Add(ctx context.Context, symbol, name, sector string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO watchlists (symbol, name, sector) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE name=VALUES(name), sector=VALUES(sector)
	`, symbol, name, sector)
	return err
}

func (r *watchlistRepo) Remove(ctx context.Context, symbol string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM watchlists WHERE symbol=?`, symbol)
	return err
}

func (r *watchlistRepo) Symbols(ctx context.Context) ([]string, error) {
	var symbols []string
	err := r.db.SelectContext(ctx, &symbols, `SELECT symbol FROM watchlists`)
	return symbols, err
}
