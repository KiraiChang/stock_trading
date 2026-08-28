package store

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/trading/backend/pkg/timeutil"
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

	// CandleDatesInRange 回傳這批標的在 [from, to) 內**實際有 K 棒的台北日期**
	// （每檔升冪、已去重），鍵是 symbol。缺漏偵測用它跟「預期交易日集合」相減
	// （見 docs/architecture.md「日 K 缺漏偵測」；原記於 issue.md I-091，已收斂）。
	//
	// **與 SymbolsWithCandleOn 的差別是問題不同**：那支問「今天抓過了沒」（布林），
	// 這支問「這段期間到底有哪幾天」——偵測要獨立於回補流程，不能掛在「本輪抓了什麼」
	// 上面。只看最新日期或本輪筆數都抓不到**視窗中段的洞**，而那正是跳過最佳化
	// （T-062）之後的主要盲點。
	//
	// **一定要帶 symbols**，理由與 SymbolsWithCandleOn 相同：複合索引
	// `(symbol, timeframe, ts)` 的首欄是 symbol，不約束它會退化成整張 candles 的
	// seq scan（live 實測 368ms vs 1.96ms）。
	//
	// **回傳 `YYYY-MM-DD` 字串而不是 time.Time**：日期是拿來跟年度日曆推導出的預期
	// 集合做集合運算的，字串比時間點更難用錯（不會有時區或時分秒殘留）。
	// 這與 ZoneIdentityRepo.ListTradingDays 的選擇一致。
	//
	// **時區轉換在 Go 端做，不寫進 SQL**：`DATE(ts)` 與 `AT TIME ZONE` 的語法與時區
	// 語意三個 driver 各不相同；查詢只用半開區間比較 timestamptz，三者一致。
	//
	// symbols 為空時回空 map，不送查詢。
	CandleDatesInRange(
		ctx context.Context, symbols []string, timeframe string, from, to time.Time,
	) (map[string][]string, error)

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

// CandleDatesInRange 見介面上的說明。
func (r *candleRepo) CandleDatesInRange(
	ctx context.Context, symbols []string, timeframe string, from, to time.Time,
) (map[string][]string, error) {
	if len(symbols) == 0 {
		return map[string][]string{}, nil
	}
	query, args, err := sqlx.In(`
		SELECT symbol, ts FROM candles
		WHERE symbol IN (?) AND timeframe=? AND ts >= ? AND ts < ?
		ORDER BY symbol, ts
	`, symbols, timeframe, from, to)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Symbol string    `db:"symbol"`
		TS     time.Time `db:"ts"`
	}
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, err
	}

	out := make(map[string][]string, len(symbols))
	// 同一天可能有多列（不同 timeframe 已被 WHERE 濾掉，但仍防個位數的重複寫入），
	// 所以用「與前一個相同就跳過」去重——輸入已依 (symbol, ts) 排序，相同日期必相鄰。
	last := make(map[string]string, len(symbols))
	for _, row := range rows {
		day := row.TS.In(timeutil.TaipeiTZ).Format("2006-01-02")
		if last[row.Symbol] == day {
			continue
		}
		last[row.Symbol] = day
		out[row.Symbol] = append(out[row.Symbol], day)
	}
	return out, nil
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
