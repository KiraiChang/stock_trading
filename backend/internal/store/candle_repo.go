package store

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const candleBulkInsertBatchSize = 50

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

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for start := 0; start < len(cs); start += candleBulkInsertBatchSize {
		end := start + candleBulkInsertBatchSize
		if end > len(cs) {
			end = len(cs)
		}
		if err := r.bulkUpsert(ctx, tx, cs[start:end]); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *candleRepo) bulkUpsert(ctx context.Context, tx *sqlx.Tx, cs []Candle) error {
	args := make([]any, 0, len(cs)*9)
	var values strings.Builder
	for i, c := range cs {
		if i > 0 {
			values.WriteString(", ")
		}
		values.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			c.Symbol,
			c.Timeframe,
			c.Open,
			c.High,
			c.Low,
			c.Close,
			c.Volume,
			c.Amount,
			c.Timestamp,
		)
	}

	query := `
		INSERT INTO candles (symbol, timeframe, open, high, low, close, volume, amount, ts)
		VALUES ` + values.String() + r.upsertSuffix()
	_, err := tx.ExecContext(ctx, r.db.Rebind(query), args...)
	return err
}

func (r *candleRepo) upsertSuffix() string {
	if r.driver == "mysql" {
		return `
			ON DUPLICATE KEY UPDATE
				open=VALUES(open), high=VALUES(high), low=VALUES(low), close=VALUES(close),
				volume=VALUES(volume), amount=VALUES(amount)`
	}
	return `
		ON CONFLICT(symbol, timeframe, ts) DO UPDATE SET
			open=excluded.open, high=excluded.high, low=excluded.low, close=excluded.close,
			volume=excluded.volume, amount=excluded.amount`
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
