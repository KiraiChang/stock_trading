package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrEmptyStockSymbolSnapshot = errors.New("stock symbol snapshot is empty")

// ErrSuspiciousStockSymbolSnapshot 代表快照涵蓋數明顯少於現有上市數，疑似來源截斷／
// 異常，為避免誤將大量真實個股標記下市，直接放棄本次同步。
var ErrSuspiciousStockSymbolSnapshot = errors.New("stock symbol snapshot is suspiciously small")

// minSnapshotListedRatio：快照涵蓋數至少要達現有上市數的這個比例，否則視為異常快照。
// 台股單日不可能下市半數以上，取 0.5 作為寬鬆的截斷偵測門檻。
const minSnapshotListedRatio = 0.5

type StockSymbolRepo interface {
	UpsertSnapshot(ctx context.Context, symbols []StockSymbol, seenAt time.Time) (StockSymbolSyncResult, error)
	Get(ctx context.Context, symbol string) (*StockSymbol, error)
	List(ctx context.Context, onlyListed bool) ([]StockSymbol, error)
	Search(ctx context.Context, opts StockSymbolSearchOptions) ([]StockSymbol, error)
}

type StockSymbolSyncResult struct {
	Seen     int `json:"seen"`
	Delisted int `json:"delisted"`
}

type StockSymbolSearchOptions struct {
	Query        string
	OnlyListed   bool
	SecurityType string
	Limit        int
}

type stockSymbolRepo struct {
	db     *sqlx.DB
	driver string
}

func NewStockSymbolRepo(db *sqlx.DB) StockSymbolRepo {
	return &stockSymbolRepo{db: db, driver: db.DriverName()}
}

func (r *stockSymbolRepo) UpsertSnapshot(ctx context.Context, symbols []StockSymbol, seenAt time.Time) (StockSymbolSyncResult, error) {
	if len(symbols) == 0 {
		return StockSymbolSyncResult{}, ErrEmptyStockSymbolSnapshot
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return StockSymbolSyncResult{}, err
	}
	defer tx.Rollback()

	priorListed, err := r.countListed(ctx, tx)
	if err != nil {
		return StockSymbolSyncResult{}, err
	}

	seen := 0
	for _, symbol := range symbols {
		if symbol.Symbol == "" {
			continue
		}
		seen++
		if err := r.upsert(ctx, tx, symbol, seenAt); err != nil {
			return StockSymbolSyncResult{}, err
		}
	}
	if seen == 0 {
		return StockSymbolSyncResult{}, ErrEmptyStockSymbolSnapshot
	}

	// 截斷／異常快照防護：涵蓋數明顯少於現有上市數時放棄本次同步，避免把大量仍在交易
	// 的個股誤標下市。首次同步（priorListed=0）不套用。
	if priorListed > 0 && float64(seen) < float64(priorListed)*minSnapshotListedRatio {
		return StockSymbolSyncResult{}, fmt.Errorf("%w: snapshot=%d, currently listed=%d", ErrSuspiciousStockSymbolSnapshot, seen, priorListed)
	}

	delisted, err := r.markMissingDelisted(ctx, tx, seenAt)
	if err != nil {
		return StockSymbolSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return StockSymbolSyncResult{}, err
	}
	return StockSymbolSyncResult{Seen: seen, Delisted: int(delisted)}, nil
}

func (r *stockSymbolRepo) countListed(ctx context.Context, tx *sqlx.Tx) (int, error) {
	var n int
	err := tx.GetContext(ctx, &n, tx.Rebind(`SELECT COUNT(*) FROM stock_symbols WHERE is_listed = ?`), true)
	return n, err
}

func (r *stockSymbolRepo) upsert(ctx context.Context, tx *sqlx.Tx, symbol StockSymbol, seenAt time.Time) error {
	listedDate := sql.NullTime(symbol.ListedDate.NullTime)
	if r.driver == "mysql" {
		_, err := tx.ExecContext(ctx, tx.Rebind(`
			INSERT INTO stock_symbols
				(symbol, name, isin_code, market, security_type, industry, cfi_code, remarks, listed_date, is_listed, last_seen_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				name=VALUES(name),
				isin_code=VALUES(isin_code),
				market=VALUES(market),
				security_type=VALUES(security_type),
				industry=VALUES(industry),
				cfi_code=VALUES(cfi_code),
				remarks=VALUES(remarks),
				listed_date=VALUES(listed_date),
				is_listed=VALUES(is_listed),
				last_seen_at=VALUES(last_seen_at),
				updated_at=VALUES(updated_at)
		`), symbol.Symbol, symbol.Name, symbol.ISINCode, symbol.Market, symbol.SecurityType,
			symbol.Industry, symbol.CFICode, symbol.Remarks, listedDate, true, seenAt, seenAt)
		return err
	}

	_, err := tx.ExecContext(ctx, tx.Rebind(`
		INSERT INTO stock_symbols
			(symbol, name, isin_code, market, security_type, industry, cfi_code, remarks, listed_date, is_listed, last_seen_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol) DO UPDATE SET
			name=excluded.name,
			isin_code=excluded.isin_code,
			market=excluded.market,
			security_type=excluded.security_type,
			industry=excluded.industry,
			cfi_code=excluded.cfi_code,
			remarks=excluded.remarks,
			listed_date=excluded.listed_date,
			is_listed=excluded.is_listed,
			last_seen_at=excluded.last_seen_at,
			updated_at=excluded.updated_at
	`), symbol.Symbol, symbol.Name, symbol.ISINCode, symbol.Market, symbol.SecurityType,
		symbol.Industry, symbol.CFICode, symbol.Remarks, listedDate, true, seenAt, seenAt)
	return err
}

func (r *stockSymbolRepo) markMissingDelisted(ctx context.Context, tx *sqlx.Tx, seenAt time.Time) (int64, error) {
	// 本次同步已把每個出現的 symbol 的 last_seen_at 更新為 seenAt；仍標記 listed 但
	// last_seen_at 落後（本次快照缺席）者即為下市。用 last_seen_at 浮水印取代逐一列舉
	// 的 NOT IN，避免大量 placeholder（SQLite 變數上限）與長查詢。
	res, err := tx.ExecContext(ctx, tx.Rebind(`
		UPDATE stock_symbols
		SET is_listed = ?, updated_at = ?
		WHERE is_listed = ? AND last_seen_at < ?
	`), false, seenAt, true, seenAt)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *stockSymbolRepo) Get(ctx context.Context, symbol string) (*StockSymbol, error) {
	var row StockSymbol
	err := r.db.GetContext(ctx, &row, r.db.Rebind(`
		SELECT id, symbol, name, isin_code, market, security_type, industry, cfi_code, remarks,
		       listed_date, is_listed, last_seen_at, created_at, updated_at
		FROM stock_symbols
		WHERE symbol = ?
	`), symbol)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *stockSymbolRepo) List(ctx context.Context, onlyListed bool) ([]StockSymbol, error) {
	var rows []StockSymbol
	query := `
		SELECT id, symbol, name, isin_code, market, security_type, industry, cfi_code, remarks,
		       listed_date, is_listed, last_seen_at, created_at, updated_at
		FROM stock_symbols
	`
	args := []any{}
	if onlyListed {
		query += ` WHERE is_listed = ?`
		args = append(args, true)
	}
	query += ` ORDER BY symbol ASC`
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("list stock symbols: %w", err)
	}
	return rows, nil
}

func (r *stockSymbolRepo) Search(ctx context.Context, opts StockSymbolSearchOptions) ([]StockSymbol, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	query := `
		SELECT id, symbol, name, isin_code, market, security_type, industry, cfi_code, remarks,
		       listed_date, is_listed, last_seen_at, created_at, updated_at
		FROM stock_symbols
		WHERE 1=1
	`
	args := []any{}
	q := strings.TrimSpace(opts.Query)
	if q != "" {
		like := "%" + strings.ToUpper(q) + "%"
		query += ` AND (UPPER(symbol) LIKE ? OR UPPER(name) LIKE ?)`
		args = append(args, like, like)
	}
	if opts.OnlyListed {
		query += ` AND is_listed = ?`
		args = append(args, true)
	}
	if strings.TrimSpace(opts.SecurityType) != "" {
		query += ` AND security_type = ?`
		args = append(args, strings.TrimSpace(opts.SecurityType))
	}
	query += ` ORDER BY CASE WHEN symbol = ? THEN 0 WHEN UPPER(symbol) LIKE ? THEN 1 ELSE 2 END, symbol ASC LIMIT ?`
	exact := strings.ToUpper(q)
	prefix := exact + "%"
	args = append(args, exact, prefix, limit)

	var rows []StockSymbol
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("search stock symbols: %w", err)
	}
	return rows, nil
}
