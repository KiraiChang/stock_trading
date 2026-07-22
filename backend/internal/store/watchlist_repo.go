package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
)

// MaxWatchedSymbols 為 WebSocket 即時監聽的檔數上限。監控清單本身可以很大
// （Phase 2 目標 ~1900 檔），但即時監聽刻意限制在少數幾檔，避免前端對
// /ws/market 訂閱過多股票、也對應這套系統「非高頻」的定位。
const MaxWatchedSymbols = 3

// ErrWatchLimitExceeded 在嘗試把第 4 檔股票設為監聽時回傳
var ErrWatchLimitExceeded = errors.New("watch limit exceeded")

// ErrWatchlistNameRequired 代表 symbol 不在股票主檔內，且呼叫端也沒有提供名稱。
var ErrWatchlistNameRequired = errors.New("watchlist name is required when symbol is not in stock symbols")

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
	var rows []watchlistRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT
			w.id, w.symbol, w.name, w.sector, w.watched,
			ss.symbol AS stock_symbol,
			ss.is_listed AS stock_is_listed,
			ss.isin_code AS stock_isin_code,
			ss.market AS stock_market,
			ss.security_type AS stock_security_type,
			ss.industry AS stock_industry,
			ss.last_seen_at AS stock_last_seen_at
		FROM watchlists w
		LEFT JOIN stock_symbols ss ON ss.symbol = w.symbol
		ORDER BY w.added_at ASC
	`)
	if err != nil {
		return nil, err
	}
	items := make([]WatchlistItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toItem())
	}
	return items, nil
}

func (r *watchlistRepo) Add(ctx context.Context, symbol, name, sector string) error {
	symbol, name, sector, err := r.resolveWatchlistFields(ctx, symbol, name, sector)
	if err != nil {
		return err
	}

	var sql string
	if r.driver == "mysql" {
		sql = `INSERT INTO watchlists (symbol, name, sector) VALUES (?, ?, ?)
		       ON DUPLICATE KEY UPDATE name=VALUES(name), sector=VALUES(sector)`
	} else {
		// SQLite 和 PostgreSQL 均支援 ON CONFLICT 語法
		sql = `INSERT INTO watchlists (symbol, name, sector) VALUES (?, ?, ?)
		       ON CONFLICT(symbol) DO UPDATE SET name=excluded.name, sector=excluded.sector`
	}
	_, err = r.db.ExecContext(ctx, r.db.Rebind(sql), symbol, name, sector)
	return err
}

func (r *watchlistRepo) Update(ctx context.Context, symbol, name, sector string) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE watchlists SET name=?, sector=? WHERE symbol=?
	`), strings.TrimSpace(name), strings.TrimSpace(sector), symbol)
	return err
}

func (r *watchlistRepo) Remove(ctx context.Context, symbol string) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM watchlists WHERE symbol=?`), symbol)
	return err
}

func (r *watchlistRepo) Symbols(ctx context.Context) ([]string, error) {
	var symbols []string
	err := r.db.SelectContext(ctx, &symbols, `SELECT symbol FROM watchlists`)
	return symbols, err
}

func (r *watchlistRepo) SetWatched(ctx context.Context, symbol string, watched bool) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
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

type watchlistRow struct {
	ID                uint32         `db:"id"`
	Symbol            string         `db:"symbol"`
	Name              string         `db:"name"`
	Sector            string         `db:"sector"`
	Watched           bool           `db:"watched"`
	StockSymbol       sql.NullString `db:"stock_symbol"`
	StockIsListed     sql.NullBool   `db:"stock_is_listed"`
	StockISINCode     sql.NullString `db:"stock_isin_code"`
	StockMarket       sql.NullString `db:"stock_market"`
	StockSecurityType sql.NullString `db:"stock_security_type"`
	StockIndustry     sql.NullString `db:"stock_industry"`
	StockLastSeenAt   sql.NullTime   `db:"stock_last_seen_at"`
}

func (r watchlistRow) toItem() WatchlistItem {
	item := WatchlistItem{
		ID:      r.ID,
		Symbol:  r.Symbol,
		Name:    r.Name,
		Sector:  r.Sector,
		Watched: r.Watched,
		StockSymbol: &WatchlistStockSymbol{
			Exists: false,
		},
	}
	if r.StockSymbol.Valid {
		item.StockSymbol = &WatchlistStockSymbol{
			Exists:       true,
			IsListed:     r.StockIsListed.Valid && r.StockIsListed.Bool,
			ISINCode:     r.StockISINCode.String,
			Market:       r.StockMarket.String,
			SecurityType: r.StockSecurityType.String,
			Industry:     r.StockIndustry.String,
			LastSeenAt:   NullTime{NullTime: r.StockLastSeenAt},
		}
	}
	return item
}

func (r *watchlistRepo) resolveWatchlistFields(ctx context.Context, symbol, name, sector string) (string, string, string, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	name = strings.TrimSpace(name)
	sector = strings.TrimSpace(sector)
	if symbol == "" {
		return "", "", "", ErrWatchlistNameRequired
	}
	if name != "" {
		return symbol, name, sector, nil
	}

	var master struct {
		Name     string `db:"name"`
		Industry string `db:"industry"`
	}
	err := r.db.GetContext(ctx, &master, r.db.Rebind(`
		SELECT name, industry FROM stock_symbols WHERE symbol = ?
	`), symbol)
	if err == nil {
		if sector == "" {
			sector = strings.TrimSpace(master.Industry)
		}
		return symbol, strings.TrimSpace(master.Name), sector, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", ErrWatchlistNameRequired
	}
	return "", "", "", err
}
