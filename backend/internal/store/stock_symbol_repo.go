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
	// ListCandidates 批次取研究用的候選標的清單（todo.md T-040 Step 1／Step 3）。
	// 與 Search **刻意分開**：Search 是 autocomplete，它把 limit 夾在 100 以內是刻意設計，
	// 撐大它會讓兩種用途互相牽制。
	ListCandidates(ctx context.Context, opts StockSymbolCandidateOptions) (StockSymbolCandidateResult, error)
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

// StockSymbolCandidateOptions 是「批次取候選清單」的條件。零值代表不限制。
type StockSymbolCandidateOptions struct {
	// SecurityTypes / Industries 為空代表不限；多值為 OR。
	SecurityTypes []string
	Industries    []string
	// ListedBefore 只留上市日早於此時間的標的（T-040 的「上市滿 5 年」規則）。
	// **listed_date 為 NULL 的一律排除**——證不出上市夠久就不該進研究母體。
	ListedBefore time.Time
	// IncludeDelisted 預設 false：研究母體不該混入已下市標的。
	IncludeDelisted bool
	// PerIndustryLimit > 0 時，每個產業最多取這麼多檔，**在該產業的代號區間內等距取樣**
	// （不是取代號最小的前 N 檔，理由見 ListCandidates 內的說明）。
	// 這是 T-040「各產業分層抽樣」與「限制單一產業佔比」的實作——半導體業有 201 檔，
	// 不設限的話抽樣會被它主導。
	//
	// **`industry = ''` 的列不受此上限約束**：那代表「沒有產業分類」而不是一個產業，
	// ETF 與權證都落在這裡。
	PerIndustryLimit int
	Limit            int
}

// StockSymbolCandidateResult 帶著 Truncated，因為呼叫端光看筆數分不出
// 「母體剛好等於上限」與「被上限砍掉」——後者會依代號順序整批砍掉高代號的產業。
type StockSymbolCandidateResult struct {
	Symbols   []StockSymbol
	Truncated bool
}

// 候選清單是研究用途，母體是全市場約 2,300 檔，所以上限給得比 Search 寬得多；
// 但仍要有上限，避免一個沒帶條件的請求把整張表撈進記憶體。
const (
	defaultCandidateLimit = 3000
	maxCandidateLimit     = 5000
)

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

const stockSymbolColumns = `id, symbol, name, isin_code, market, security_type, industry, cfi_code, remarks,
	       listed_date, is_listed, last_seen_at, created_at, updated_at`

func (r *stockSymbolRepo) ListCandidates(ctx context.Context, opts StockSymbolCandidateOptions) (StockSymbolCandidateResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultCandidateLimit
	}
	if limit > maxCandidateLimit {
		limit = maxCandidateLimit
	}

	where := []string{"1=1"}
	args := []any{}
	if !opts.IncludeDelisted {
		where = append(where, "is_listed = ?")
		args = append(args, true)
	}
	if len(opts.SecurityTypes) > 0 {
		where = append(where, "security_type IN (?)")
		args = append(args, opts.SecurityTypes)
	}
	if len(opts.Industries) > 0 {
		where = append(where, "industry IN (?)")
		args = append(args, opts.Industries)
	}
	if !opts.ListedBefore.IsZero() {
		where = append(where, "listed_date IS NOT NULL AND listed_date <= ?")
		args = append(args, opts.ListedBefore)
	}
	filter := strings.Join(where, " AND ")

	var query string
	if opts.PerIndustryLimit > 0 {
		// 三種 engine 都支援 window function（sqlite ≥ 3.25 / mysql 8 / postgres）。
		//
		// **系統性取樣而不是取前 N 檔**：`ORDER BY symbol` 取最小的 N 個看似單純，
		// 但台股代號大致依上市資歷編配，取前 N 等於每個產業永遠只拿最老、最大的那幾檔
		// （半導體業永遠是 2330／2454），中小型股完全沒有機會入選。Step 1 要量的是
		// 全市場的 ATR% 分佈，而大型股波動系統性偏低，取前 N 會把分佈整個往低波動端拉。
		//
		// `((rn - 1) * k) % n < k` 會在 1..n 中均勻挑出 k 筆（Bresenham 式的等距取樣），
		// 跨越整個資歷區間，且仍是**決定性的**——同條件每次拿到同一批，Step 1 的量測才能重現。
		// n <= k 時條件恆真，整個產業全取，行為與原本一致。
		//
		// **industry = '' 的列不套用上限**：那是「沒有產業分類」而不是一個產業。
		// ETF 與權證的 industry 都是空字串（NOT NULL DEFAULT ''），若把它們當成單一產業，
		// per_industry=9 會讓 354 檔 ETF 只剩 9 檔——而 ETF 是目前唯一填得進 LOW bucket
		// 的類型，Step 1 的分佈會建立在殘缺樣本上。
		query = `SELECT ` + stockSymbolColumns + ` FROM (
			SELECT ` + stockSymbolColumns + `,
			       ROW_NUMBER() OVER (PARTITION BY industry ORDER BY symbol) AS industry_rank,
			       COUNT(*)     OVER (PARTITION BY industry)                 AS industry_total
			FROM stock_symbols WHERE ` + filter + `
		) ranked WHERE industry = '' OR ((industry_rank - 1) * ?) % industry_total < ?`
		args = append(args, opts.PerIndustryLimit, opts.PerIndustryLimit)
	} else {
		query = `SELECT ` + stockSymbolColumns + ` FROM stock_symbols WHERE ` + filter
	}
	// 多取一筆用來分辨「剛好等於上限」與「被截斷」——呼叫端拿到 count == limit 時
	// 無法自行判斷，而截斷會依代號順序砍掉整批高代號的產業，正是 PerIndustryLimit
	// 要消除的偏斜。
	query += ` ORDER BY symbol ASC LIMIT ?`
	args = append(args, limit+1)

	// sqlx.In 展開 IN (?) 的切片參數；要先 In 再 Rebind，順序反過來會把展開後的
	// 佔位符再轉一次。
	query, args, err := sqlx.In(query, args...)
	if err != nil {
		return StockSymbolCandidateResult{}, fmt.Errorf("expand stock symbol candidate query: %w", err)
	}

	var rows []StockSymbol
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return StockSymbolCandidateResult{}, fmt.Errorf("list stock symbol candidates: %w", err)
	}

	truncated := len(rows) > limit
	if truncated {
		rows = rows[:limit]
	}
	// SelectContext 查無資料時留下 nil slice，序列化成 JSON 是 null 而不是 []。
	// handler 的 symbols 是用 make 建的、永遠是 []，兩個清單欄位形狀不一致會讓前端
	// 對其中一個做 .map() 直接爆掉。
	if rows == nil {
		rows = []StockSymbol{}
	}
	return StockSymbolCandidateResult{Symbols: rows, Truncated: truncated}, nil
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
