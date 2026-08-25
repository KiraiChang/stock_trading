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
	// SymbolsWithCandleOn 從 symbols 裡挑出「在 day 當天已有該 timeframe K 棒」的標的（升冪）。
	//
	// **一句查詢，不是逐檔 GetLatest**（見 docs/architecture.md 的「日 K 維護……會跳過
	// 『今天已有日 K』的標的」）：呼叫端
	// `runEvaluationUniverseSync` 要對整個評估標的池（實測 135 檔）判斷「今天抓過了沒」，
	// 逐檔查是 N+1，而且成本隨池成長。
	//
	// **一定要帶 symbols，不能只查 timeframe ＋ ts**（2026-08-25 review 後改）：
	// 複合索引是 `(symbol, timeframe, ts)`，symbol 是首欄；不約束它的話 PostgreSQL 16
	// 沒有 skip scan 可用，只能整張 candles 掃過去。live 實測差 188 倍：
	// 不帶 symbols 是 Parallel Seq Scan、掃掉 211,837 列、368ms；帶 symbols 是
	// Index Only Scan（Heap Fetches: 0）、1.96ms。而呼叫端本來就有那份清單，
	// 要問的問題也正是「**這些**標的今天抓過了沒」。
	//
	// **day 是台北時區的日界**（`timeutil.TodayTaipei()`），比較用半開區間
	// `[day, day+1d)`。刻意不寫 `DATE(ts)` 或 `AT TIME ZONE`——那兩者的語法與時區
	// 語意三個 driver 各不相同，而半開區間只靠 timestamptz 的大小比較，三者一致。
	//
	// symbols 為空時回空集合，不送查詢。
	SymbolsWithCandleOn(ctx context.Context, symbols []string, timeframe string, day time.Time) ([]string, error)

	// 還原係數的維護（見 docs/todo.md T-042）。實作在 corporate_action_repo.go，
	// 與事件表的操作放在一起比較好讀。
	ApplyAdjFactors(ctx context.Context, symbol string, ranges []AdjFactorRange) error
	CountUnadjustedBefore(ctx context.Context, symbol string, before time.Time) (int, error)
	// Symbols 回傳 candles 內所有相異 symbol。還原係數要涵蓋**所有有價格歷史的標的**，
	// 不能只看 watchlist——評估標的池（T-040）的標的不在 watchlist 裡，
	// 漏掉會讓它們「分割有還原、除權息沒有」。
	Symbols(ctx context.Context) ([]string, error)
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
		SELECT id, symbol, timeframe, open, high, low, close, volume, amount, ts, adj_factor, vol_factor
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
		SELECT id, symbol, timeframe, open, high, low, close, volume, amount, ts, adj_factor, vol_factor
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
		SELECT id, symbol, timeframe, open, high, low, close, volume, amount, ts, adj_factor, vol_factor
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

// SymbolsWithCandleOn 見介面上的說明。
func (r *candleRepo) SymbolsWithCandleOn(
	ctx context.Context, symbols []string, timeframe string, day time.Time,
) ([]string, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	// sqlx.In 展開 IN 的佔位符後仍要 Rebind——展開出來的是 `?`，postgres 要的是 `$n`。
	query, args, err := sqlx.In(`
		SELECT DISTINCT symbol FROM candles
		WHERE symbol IN (?) AND timeframe=? AND ts >= ? AND ts < ?
		ORDER BY symbol
	`, symbols, timeframe, day, day.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	var rows []string
	err = r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...)
	return rows, err
}
