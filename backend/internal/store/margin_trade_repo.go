package store

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const marginTradeBulkBatchSize = 50

type MarginTradeRepo interface {
	Upsert(ctx context.Context, t *MarginTrade) error
	BulkUpsert(ctx context.Context, ts []MarginTrade) error
	GetByDate(ctx context.Context, symbol string, date time.Time) (*MarginTrade, error)
	GetRange(ctx context.Context, symbol string, from, to time.Time) ([]MarginTrade, error)
	GetLatestN(ctx context.Context, symbol string, n int) ([]MarginTrade, error)
}

type marginTradeRepo struct {
	db     *sqlx.DB
	driver string
}

func NewMarginTradeRepo(db *sqlx.DB) MarginTradeRepo {
	return &marginTradeRepo{db: db, driver: db.DriverName()}
}

const marginTradeColumns = `id, symbol, trade_date, margin_balance, margin_change, short_balance, short_change, margin_usage_rate, short_usage_rate, created_at, updated_at`

func (r *marginTradeRepo) upsertSQL() string {
	if r.driver == "mysql" {
		return `
			INSERT INTO margin_trades (symbol, trade_date, margin_balance, margin_change, short_balance, short_change, margin_usage_rate, short_usage_rate)
			VALUES (:symbol, :trade_date, :margin_balance, :margin_change, :short_balance, :short_change, :margin_usage_rate, :short_usage_rate)
			ON DUPLICATE KEY UPDATE
				margin_balance=VALUES(margin_balance), margin_change=VALUES(margin_change),
				short_balance=VALUES(short_balance), short_change=VALUES(short_change),
				margin_usage_rate=VALUES(margin_usage_rate), short_usage_rate=VALUES(short_usage_rate),
				updated_at=CURRENT_TIMESTAMP`
	}
	return `
		INSERT INTO margin_trades (symbol, trade_date, margin_balance, margin_change, short_balance, short_change, margin_usage_rate, short_usage_rate)
		VALUES (:symbol, :trade_date, :margin_balance, :margin_change, :short_balance, :short_change, :margin_usage_rate, :short_usage_rate)
		ON CONFLICT(symbol, trade_date) DO UPDATE SET
			margin_balance=excluded.margin_balance, margin_change=excluded.margin_change,
			short_balance=excluded.short_balance, short_change=excluded.short_change,
			margin_usage_rate=excluded.margin_usage_rate, short_usage_rate=excluded.short_usage_rate,
			updated_at=CURRENT_TIMESTAMP`
}

func (r *marginTradeRepo) Upsert(ctx context.Context, t *MarginTrade) error {
	_, err := r.db.NamedExecContext(ctx, r.upsertSQL(), t)
	return err
}

func (r *marginTradeRepo) BulkUpsert(ctx context.Context, ts []MarginTrade) error {
	if len(ts) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for start := 0; start < len(ts); start += marginTradeBulkBatchSize {
		end := start + marginTradeBulkBatchSize
		if end > len(ts) {
			end = len(ts)
		}
		if err := r.bulkUpsert(ctx, tx, ts[start:end]); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *marginTradeRepo) bulkUpsert(ctx context.Context, tx *sqlx.Tx, ts []MarginTrade) error {
	args := make([]any, 0, len(ts)*8)
	var values strings.Builder
	for i, t := range ts {
		if i > 0 {
			values.WriteString(", ")
		}
		values.WriteString("(?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args, t.Symbol, t.TradeDate, t.MarginBalance, t.MarginChange, t.ShortBalance, t.ShortChange, t.MarginUsageRate, t.ShortUsageRate)
	}

	query := `
		INSERT INTO margin_trades (symbol, trade_date, margin_balance, margin_change, short_balance, short_change, margin_usage_rate, short_usage_rate)
		VALUES ` + values.String() + r.upsertSuffix()
	_, err := tx.ExecContext(ctx, r.db.Rebind(query), args...)
	return err
}

func (r *marginTradeRepo) upsertSuffix() string {
	if r.driver == "mysql" {
		return `
			ON DUPLICATE KEY UPDATE
				margin_balance=VALUES(margin_balance), margin_change=VALUES(margin_change),
				short_balance=VALUES(short_balance), short_change=VALUES(short_change),
				margin_usage_rate=VALUES(margin_usage_rate), short_usage_rate=VALUES(short_usage_rate),
				updated_at=CURRENT_TIMESTAMP`
	}
	return `
		ON CONFLICT(symbol, trade_date) DO UPDATE SET
			margin_balance=excluded.margin_balance, margin_change=excluded.margin_change,
			short_balance=excluded.short_balance, short_change=excluded.short_change,
			margin_usage_rate=excluded.margin_usage_rate, short_usage_rate=excluded.short_usage_rate,
			updated_at=CURRENT_TIMESTAMP`
}

func (r *marginTradeRepo) GetByDate(ctx context.Context, symbol string, date time.Time) (*MarginTrade, error) {
	var t MarginTrade
	sql := r.db.Rebind(`SELECT ` + marginTradeColumns + ` FROM margin_trades WHERE symbol=? AND trade_date=?`)
	if err := r.db.GetContext(ctx, &t, sql, symbol, date); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *marginTradeRepo) GetRange(ctx context.Context, symbol string, from, to time.Time) ([]MarginTrade, error) {
	var rows []MarginTrade
	sql := r.db.Rebind(`
		SELECT ` + marginTradeColumns + `
		FROM margin_trades
		WHERE symbol=? AND trade_date BETWEEN ? AND ?
		ORDER BY trade_date ASC
	`)
	err := r.db.SelectContext(ctx, &rows, sql, symbol, from, to)
	return rows, err
}

func (r *marginTradeRepo) GetLatestN(ctx context.Context, symbol string, n int) ([]MarginTrade, error) {
	var rows []MarginTrade
	sql := r.db.Rebind(`
		SELECT ` + marginTradeColumns + `
		FROM margin_trades
		WHERE symbol=?
		ORDER BY trade_date DESC
		LIMIT ?
	`)
	if err := r.db.SelectContext(ctx, &rows, sql, symbol, n); err != nil {
		return nil, err
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}
