package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

type CandleRepo interface {
	Insert(ctx context.Context, c *Candle) error
	BulkInsert(ctx context.Context, cs []Candle) error
	GetLatestN(ctx context.Context, symbol, timeframe string, n int) ([]Candle, error)
	GetRange(ctx context.Context, symbol, timeframe string, from, to time.Time) ([]Candle, error)
	GetLatest(ctx context.Context, symbol, timeframe string) (*Candle, error)
}

type candleRepo struct {
	db     *sqlx.DB
	driver string
}

func NewCandleRepo(db *sqlx.DB) CandleRepo {
	return &candleRepo{db: db, driver: db.DriverName()}
}

// upsertSQL 依 driver 回傳相容的 UPSERT 語法
func (r *candleRepo) upsertSQL() string {
	if r.driver == "mysql" {
		return `
			INSERT INTO candles (symbol, timeframe, open, high, low, close, volume, amount, ts)
			VALUES (:symbol, :timeframe, :open, :high, :low, :close, :volume, :amount, :ts)
			ON DUPLICATE KEY UPDATE
				open=VALUES(open), high=VALUES(high), low=VALUES(low), close=VALUES(close),
				volume=VALUES(volume), amount=VALUES(amount)`
	}
	// SQLite 和 PostgreSQL 均支援 ON CONFLICT 語法
	return `
		INSERT INTO candles (symbol, timeframe, open, high, low, close, volume, amount, ts)
		VALUES (:symbol, :timeframe, :open, :high, :low, :close, :volume, :amount, :ts)
		ON CONFLICT(symbol, timeframe, ts) DO UPDATE SET
			open=excluded.open, high=excluded.high, low=excluded.low, close=excluded.close,
			volume=excluded.volume, amount=excluded.amount`
}

func (r *candleRepo) Insert(ctx context.Context, c *Candle) error {
	_, err := r.db.NamedExecContext(ctx, r.upsertSQL(), c)
	return err
}

func (r *candleRepo) BulkInsert(ctx context.Context, cs []Candle) error {
	if len(cs) == 0 {
		return nil
	}
	sql := r.upsertSQL()
	for _, c := range cs {
		if _, err := r.db.NamedExecContext(ctx, sql, c); err != nil {
			return err
		}
	}
	return nil
}

func (r *candleRepo) GetLatestN(ctx context.Context, symbol, timeframe string, n int) ([]Candle, error) {
	var rows []Candle
	sql := r.db.Rebind(`
		SELECT id, symbol, timeframe, open, high, low, close, volume, amount, ts
		FROM candles
		WHERE symbol=? AND timeframe=?
		ORDER BY ts DESC
		LIMIT ?
	`)
	if err := r.db.SelectContext(ctx, &rows, sql, symbol, timeframe, n); err != nil {
		return nil, err
	}
	// 反轉回升冪排序，供指標計算使用
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}

func (r *candleRepo) GetRange(ctx context.Context, symbol, timeframe string, from, to time.Time) ([]Candle, error) {
	var rows []Candle
	sql := r.db.Rebind(`
		SELECT id, symbol, timeframe, open, high, low, close, volume, amount, ts
		FROM candles
		WHERE symbol=? AND timeframe=? AND ts BETWEEN ? AND ?
		ORDER BY ts ASC
	`)
	err := r.db.SelectContext(ctx, &rows, sql, symbol, timeframe, from, to)
	return rows, err
}

func (r *candleRepo) GetLatest(ctx context.Context, symbol, timeframe string) (*Candle, error) {
	var c Candle
	sql := r.db.Rebind(`
		SELECT id, symbol, timeframe, open, high, low, close, volume, amount, ts
		FROM candles
		WHERE symbol=? AND timeframe=?
		ORDER BY ts DESC
		LIMIT 1
	`)
	if err := r.db.GetContext(ctx, &c, sql, symbol, timeframe); err != nil {
		return nil, err
	}
	return &c, nil
}
