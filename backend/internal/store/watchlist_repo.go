package store

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
)

// MaxWatchedSymbols 為 WebSocket 即時監聽的檔數上限。監控清單本身可以很大
// （Phase 2 目標 ~1900 檔），但即時監聽刻意限制在少數幾檔，避免前端對
// /ws/market 訂閱過多股票、也對應這套系統「非高頻」的定位。
const MaxWatchedSymbols = 3

// ErrWatchLimitExceeded 在嘗試把第 4 檔股票設為監聽時回傳
var ErrWatchLimitExceeded = errors.New("watch limit exceeded")

type WatchlistRepo interface {
	GetAll(ctx context.Context) ([]WatchlistItem, error)
	Add(ctx context.Context, symbol, name, sector string) error
	Update(ctx context.Context, symbol, name, sector string) error
	Remove(ctx context.Context, symbol string) error
	Symbols(ctx context.Context) ([]string, error)
	// SetWatched 切換某檔股票是否要即時監聽；watched=true 時若目前監聽數已達
	// MaxWatchedSymbols 上限，回傳 ErrWatchLimitExceeded。
	SetWatched(ctx context.Context, symbol string, watched bool) error
}

type watchlistRepo struct {
	db     *sqlx.DB
	driver string
}

func NewWatchlistRepo(db *sqlx.DB) WatchlistRepo {
	return &watchlistRepo{db: db, driver: db.DriverName()}
}

func (r *watchlistRepo) GetAll(ctx context.Context) ([]WatchlistItem, error) {
	var rows []WatchlistItem
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, symbol, name, sector, watched FROM watchlists ORDER BY added_at ASC
	`)
	return rows, err
}

func (r *watchlistRepo) Add(ctx context.Context, symbol, name, sector string) error {
	var sql string
	if r.driver == "mysql" {
		sql = `INSERT INTO watchlists (symbol, name, sector) VALUES (?, ?, ?)
		       ON DUPLICATE KEY UPDATE name=VALUES(name), sector=VALUES(sector)`
	} else {
		// SQLite 和 PostgreSQL 均支援 ON CONFLICT 語法
		sql = `INSERT INTO watchlists (symbol, name, sector) VALUES (?, ?, ?)
		       ON CONFLICT(symbol) DO UPDATE SET name=excluded.name, sector=excluded.sector`
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(sql), symbol, name, sector)
	return err
}

func (r *watchlistRepo) Update(ctx context.Context, symbol, name, sector string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE watchlists SET name=?, sector=? WHERE symbol=?
	`), name, sector, symbol)
	return err
}

func (r *watchlistRepo) Remove(ctx context.Context, symbol string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM watchlists WHERE symbol=?`), symbol)
	return err
}

func (r *watchlistRepo) Symbols(ctx context.Context) ([]string, error) {
	var symbols []string
	err := r.db.SelectContext(ctx, &symbols, `SELECT symbol FROM watchlists`)
	return symbols, err
}

func (r *watchlistRepo) SetWatched(ctx context.Context, symbol string, watched bool) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if watched {
		// 用 bind 參數而非字面 1/0：postgres 的 watched 是原生 BOOLEAN，
		// "watched = 1" 這種字面整數比較在 postgres 會直接報型別錯誤
		var count int
		if err := tx.GetContext(ctx, &count, tx.Rebind(`
			SELECT COUNT(*) FROM watchlists WHERE watched = ? AND symbol != ?
		`), true, symbol); err != nil {
			return err
		}
		if count >= MaxWatchedSymbols {
			return ErrWatchLimitExceeded
		}
	}

	if _, err := tx.ExecContext(ctx, tx.Rebind(`
		UPDATE watchlists SET watched = ? WHERE symbol = ?
	`), watched, symbol); err != nil {
		return err
	}

	return tx.Commit()
}
