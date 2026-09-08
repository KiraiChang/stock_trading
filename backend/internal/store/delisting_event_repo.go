package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// 終止上市事件層與投影 provenance 的存取（計畫書 docs/todo.md T-071，migration 076）。
//
// **兩層設計**：delisting_events 記歷史事實（同一代號可有多代），
// stock_symbols 是 current-state 主檔（一代號一列）。分層的理由見 migration 076 的註解。

// DelistingEvent 是「來源 S 說代號 X 在日期 D 終止上市」這件事。
type DelistingEvent struct {
	ID           uint64    `db:"id"`
	Source       string    `db:"source"`
	Symbol       string    `db:"symbol"`
	CompanyName  string    `db:"company_name"`
	DelistedDate time.Time `db:"delisted_date"`
	FirstSeenAt  time.Time `db:"first_seen_at"`
	LastSeenAt   time.Time `db:"last_seen_at"`
	// 非 NULL = 已從來源消失。**不參與投影**，但保留供人工判讀（永不 DELETE）。
	MissingFromSourceAt NullTime `db:"missing_from_source_at"`
}

// Key 是事件的身分鍵，對應 UNIQUE(source, symbol, delisted_date)。
func (e DelistingEvent) Key() DelistingEventKey {
	return DelistingEventKey{Source: e.Source, Symbol: e.Symbol, DelistedDate: DateKey(e.DelistedDate)}
}

// DateKey 把 DATE 欄位的值轉成身分鍵用的日期字串。
//
// ⛔ **不得先 `.UTC()` 再取日期。** `delisted_date` 是 **DATE**（沒有時刻、沒有時區），
// 但各 driver 交還的 `time.Time` 帶的 location 不同：
//
//	postgres（pgx）      2026-09-01 00:00:00 UTC
//	mysql（parseTime ＋ loc=Asia/Taipei，見 config.yaml 的 DSN 範例）
//	                     2026-09-01 00:00:00 +08:00
//
// 對後者做 `.UTC()` 會得到 **2026-08-31** 16:00——身分鍵、事件 id 查找與
// 「同一天」比對會整批**差一天**，而且不會有任何東西報錯。
// 所以一律取**掛鐘日期**（driver 給的那一天就是 DB 裡的那一天）。
func DateKey(t time.Time) string { return t.Format("2006-01-02") }

// SameCalendarDay 比較兩個 DATE 值是否為同一天。⛔ 同樣不做時區換算，理由見 DateKey。
func SameCalendarDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// DelistingEventKey 用字串日期而不是 time.Time：後者帶時區與單調時鐘，
// 當 map key 會讓「同一天」比不相等。
type DelistingEventKey struct {
	Source       string
	Symbol       string
	DelistedDate string
}

// DelistingSourceSnapshot 是一次成功抓取的來源 metadata，縮水防護的基準。
type DelistingSourceSnapshot struct {
	ID     uint64 `db:"id"`
	Source string `db:"source"`
	// nullable：job_runs 只留 30 天，而快照必須長期保存（ON DELETE SET NULL）。
	JobRunID    sql.NullInt64 `db:"job_run_id"`
	FetchedAt   time.Time     `db:"fetched_at"`
	RowCount    int           `db:"row_count"`
	ContentHash string        `db:"content_hash"`
	Accepted    bool          `db:"accepted"`
}

// SymbolProjectionState 是投影仲裁需要的主檔快照。
//
// ⚠️ **這是「讀取當時」的狀態**，最終寫入時要拿它整組去做 CAS——
// name / market / security_type / listed_date 都是 candidate 的輸入，
// 讀完之後被改掉的話，記憶體裡的仲裁結果就已經過期。
type SymbolProjectionState struct {
	Symbol          string        `db:"symbol"`
	Name            string        `db:"name"`
	Market          string        `db:"market"`
	SecurityType    string        `db:"security_type"`
	ListedDate      NullTime      `db:"listed_date"`
	IsListed        bool          `db:"is_listed"`
	DelistedDate    NullTime      `db:"delisted_date"`
	DelistedEventID sql.NullInt64 `db:"delisted_event_id"`
}

// JobOwned 表示這筆投影是 delisting_reconcile 寫的（可被它替換／清除）。
// ⚠️ DelistedDate 非 NULL 但 DelistedEventID 為 NULL 是**人工值**，該 job 永不自動改動。
func (s SymbolProjectionState) JobOwned() bool { return s.DelistedEventID.Valid }

// Manual 表示這是人工填的日期。
func (s SymbolProjectionState) Manual() bool {
	return s.DelistedDate.Valid && !s.DelistedEventID.Valid
}

// DelistingRepo 只暴露一個交易入口。
//
// ⚠️ **三種寫入必須在同一個 transaction 內**：事件 upsert／消失標記、主檔投影、
// accepted 快照。第三項尤其不能落在交易外——它是下一輪縮水判斷的基準，
// 「事件更新了但快照沒寫」會讓下一輪拿舊基準比新事件。
type DelistingRepo interface {
	WithTx(ctx context.Context, fn func(DelistingTx) error) error
}

// DelistingTx 是交易內可用的操作。
type DelistingTx interface {
	// LastAcceptedSnapshot 取上一筆 accepted 快照，縮水防護的基準。
	//
	// ⛔ **一律 ORDER BY id DESC**：兩次連續執行可能落在同一個 fetched_at
	// （秒級精度），只靠時間戳排序就變成未定義，基準會不確定。
	LastAcceptedSnapshot(source string) (*DelistingSourceSnapshot, error)
	InsertSnapshot(snap DelistingSourceSnapshot) error

	// LoadEvents 取該來源的**全部**事件（含已標記消失的）。
	//
	// ⚠️ 改名與重現的計數**必須**靠它：主 upsert 每輪都會更新 last_seen_at，
	// 拿 RowsAffected 當計數的話，一筆都沒更正也會算成「全部都變了」→ 永久 partial。
	// 而且三種 engine 的 affected-row 語意還各不相同（MySQL 值未變回 0、更新回 2、
	// 插入回 1；postgres 一律 1），拿它當語意來源本身就不可移植。
	LoadEvents(source string) ([]DelistingEvent, error)
	UpsertEvent(ev DelistingEvent, seenAt time.Time) error

	// MarkMissing 把不在 present 裡的事件標記為已消失，回傳**首次缺席**的筆數。
	//
	// ✅ 這一條**可以**用 affected rows：SQL 帶 `WHERE missing_from_source_at IS NULL`，
	// predicate 自己保證了「只有真的發生 NULL → timestamp 轉換的列才會被更新」。
	// 持續缺席的列不會被碰到，也就不會每輪都算一次更正。
	MarkMissing(source string, present map[DelistingEventKey]struct{}, at time.Time) (int64, error)

	// SymbolStates 讀主檔快照。⛔ **用一般 SELECT，不可 FOR UPDATE**——
	// 鎖住之後 ISIN／人工更新只能等到本交易結束，CAS 就永遠不會落空，
	// X／RX 變成不可達，那些路徑也就測不到。鎖只出現在最終寫入點。
	SymbolStates(symbols []string) (map[string]SymbolProjectionState, error)
	// DelistedListedStocks 取「已下市的上市股票」——方向二（R／RX／A）的母體。
	//
	// ⚠️ 母體必須限縮 market/security_type：不限縮的話 4445 筆權證會全部命中，
	// 而權證是**到期**不是終止上市，根本不在這份 CSV 裡。
	DelistedListedStocks() ([]SymbolProjectionState, error)

	// Project 寫入／替換投影。回傳 false 代表 CAS 落空（歸 X）。
	Project(prior SymbolProjectionState, date time.Time, eventID uint64, now time.Time) (bool, error)
	// ConfirmProjection 用在「目標值與現值相同」時。
	//
	// ⛔ **不可改發 no-op UPDATE 再看 affected rows**：MySQL 把欄位更新成相同值時
	// 通常回報 0 affected rows，於是一個穩定且完全正確的投影會**每天被誤判成衝突**。
	// 這裡改用帶同一組 CAS 條件的 current read（pg/mysql 是 FOR UPDATE）。
	ConfirmProjection(prior SymbolProjectionState) (bool, error)
	// Revoke 清除投影。回傳 false 代表 CAS 落空（歸 RX）。
	//
	// ⛔ CAS 條件與 Project **完全相同**：R 的判定依據裡有 name/market/type/listed_date，
	// 只比 ownership 的話，「因名稱不符而準備撤銷、期間名稱被修正成相符」
	// 仍會成功清除那筆已經重新成立的投影。
	Revoke(prior SymbolProjectionState, now time.Time) (bool, error)
}

type delistingRepo struct {
	db     *sqlx.DB
	driver string
}

func NewDelistingRepo(db *sqlx.DB) DelistingRepo {
	return &delistingRepo{db: db, driver: db.DriverName()}
}

// delistingRollbackTimeout 是「交易已經失敗，還要把它收乾淨」的獨立預算。
// ⚠️ 走 context.WithoutCancel：呼叫端的 ctx 逾時／取消之後仍然要 rollback，
// 否則會把**還開著交易**的 connection 放回 pool。
const delistingRollbackTimeout = 5 * time.Second

func (r *delistingRepo) WithTx(ctx context.Context, fn func(DelistingTx) error) error {
	// SQLite 走 BEGIN IMMEDIATE 的專用路徑，理由見 withImmediateTx。
	// ⛔ 其他 engine **維持 BeginTxx 不動**：PostgreSQL／MySQL 沒有這個問題，
	// 而手動交易要自己管 connection 生命週期，沒有理由讓它們一起承擔。
	if r.driver == "sqlite" {
		return r.withImmediateTx(ctx, fn)
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(&delistingTx{ctx: ctx, tx: tx, driver: r.driver}); err != nil {
		return err
	}
	return tx.Commit()
}

// withImmediateTx 是 SQLite 專用路徑：在**任何讀取之前**用 `BEGIN IMMEDIATE`
// 取得 writer reservation。
//
// ⛔ **不能用 `BeginTxx`**（deferred transaction）：本交易是「先讀後寫」
// （`LastAcceptedSnapshot` / `LoadEvents` → upsert / 投影 / 快照），而 SQLite 在
// 「已經讀過、要升級成 writer」的當下**不呼叫 busy handler**——重試會破壞它已經拿到的
// 讀快照——直接回 `SQLITE_BUSY`。於是 DSN 上的 `busy_timeout=5000` 對這個交易
// **完全沒有作用**（實測 0.06 秒就失敗，原 `issue.md` I-111）。
// `BEGIN IMMEDIATE` 把等待移到交易開頭，busy handler 才生效
// （見 https://www.sqlite.org/lang_transaction.html）。
//
// **為什麼這裡值得付這個複雜度**：這個交易成功時必然寫入（事件＋投影＋快照），
// 本來就不是唯讀交易；而網路抓取已經在交易外完成，提前取得 write lock 增加的
// 鎖定時間有限。專案的 `backend/config.yaml` 預設仍是 SQLite，所以這不只是測試問題。
//
// ⚠️ **必須綁在單一 `*sqlx.Conn` 上**：`BEGIN`／所有查詢／`COMMIT`／`ROLLBACK`
// 要落在同一條 physical connection，否則 pool 可能把後續語句派到別條連線，
// 那些語句就會跑在交易外面。
func (r *delistingRepo) withImmediateTx(ctx context.Context, fn func(DelistingTx) error) (err error) {
	conn, err := r.db.Connx(ctx)
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		// 還沒開始交易，連線可以直接還回 pool。
		conn.Close()
		return fmt.Errorf("delisting: begin immediate: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			rbCtx, cancel := context.WithTimeout(
				context.WithoutCancel(ctx), delistingRollbackTimeout)
			defer cancel()
			if _, rerr := conn.ExecContext(rbCtx, "ROLLBACK"); rerr != nil {
				// ⛔ rollback 失敗代表這條連線**可能還開著交易**，還回 pool 的話
				// 下一個使用者會繼承那個交易。用 Raw + driver.ErrBadConn 讓
				// database/sql 直接丟棄它（重開一條的成本微不足道）。
				discardBadConn(conn)
				err = errors.Join(err, fmt.Errorf("delisting: rollback failed: %w", rerr))
			}
		}
		conn.Close()
	}()

	if ferr := fn(&delistingTx{ctx: ctx, tx: conn, driver: r.driver}); ferr != nil {
		return ferr
	}
	if _, cerr := conn.ExecContext(ctx, "COMMIT"); cerr != nil {
		// ⚠️ 這裡**不設 committed**：COMMIT 失敗時交易通常仍是開著的
		// （例如撞上 SQLITE_BUSY），要靠上面的 defer 收掉。
		return fmt.Errorf("delisting: commit: %w", cerr)
	}
	committed = true
	return nil
}

// discardBadConn 讓 database/sql 丟棄這條連線而不是還回 pool。
// `Raw` 的 callback 回 driver.ErrBadConn 就是官方的作法。
func discardBadConn(conn *sqlx.Conn) {
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
}

// delistingExec 是交易內用得到的**最小**介面。
// `*sqlx.Tx`（其他 engine）與 `*sqlx.Conn`（SQLite 的 BEGIN IMMEDIATE 路徑）都滿足它，
// 所以下面每一個方法都不必知道自己跑在哪一種交易上。
type delistingExec interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Rebind(query string) string
}

type delistingTx struct {
	ctx    context.Context
	tx     delistingExec
	driver string
}

func (t *delistingTx) LastAcceptedSnapshot(source string) (*DelistingSourceSnapshot, error) {
	var snap DelistingSourceSnapshot
	err := t.tx.GetContext(t.ctx, &snap, t.tx.Rebind(`
		SELECT id, source, job_run_id, fetched_at, row_count, content_hash, accepted
		FROM delisting_source_snapshots
		WHERE source = ? AND accepted = ?
		ORDER BY id DESC
		LIMIT 1
	`), source, true)
	if err == sql.ErrNoRows {
		return nil, nil // bootstrap：還沒有任何 accepted 快照
	}
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

func (t *delistingTx) InsertSnapshot(snap DelistingSourceSnapshot) error {
	_, err := t.tx.ExecContext(t.ctx, t.tx.Rebind(`
		INSERT INTO delisting_source_snapshots (source, job_run_id, fetched_at, row_count, content_hash, accepted)
		VALUES (?, ?, ?, ?, ?, ?)
	`), snap.Source, snap.JobRunID, snap.FetchedAt, snap.RowCount, snap.ContentHash, snap.Accepted)
	return err
}

func (t *delistingTx) LoadEvents(source string) ([]DelistingEvent, error) {
	var rows []DelistingEvent
	err := t.tx.SelectContext(t.ctx, &rows, t.tx.Rebind(`
		SELECT id, source, symbol, company_name, delisted_date,
		       first_seen_at, last_seen_at, missing_from_source_at
		FROM delisting_events
		WHERE source = ?
	`), source)
	return rows, err
}

func (t *delistingTx) UpsertEvent(ev DelistingEvent, seenAt time.Time) error {
	// ⛔ missing_from_source_at 一定要設回 NULL。少了這一句，任何一次來源暫時性缺漏
	// （TWSE 當天輸出異常、我方解析失誤）都會讓該事件**永久失去投影資格**——
	// 重新出現時只更新 last_seen_at、標記還在、投影仍被擋掉，而且不會有東西報錯。
	if t.driver == "mysql" {
		_, err := t.tx.ExecContext(t.ctx, t.tx.Rebind(`
			INSERT INTO delisting_events
				(source, symbol, company_name, delisted_date, first_seen_at, last_seen_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				company_name=VALUES(company_name),
				last_seen_at=VALUES(last_seen_at),
				missing_from_source_at=NULL,
				updated_at=VALUES(updated_at)
		`), ev.Source, ev.Symbol, ev.CompanyName, ev.DelistedDate, seenAt, seenAt, seenAt)
		return err
	}
	_, err := t.tx.ExecContext(t.ctx, t.tx.Rebind(`
		INSERT INTO delisting_events
			(source, symbol, company_name, delisted_date, first_seen_at, last_seen_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source, symbol, delisted_date) DO UPDATE SET
			company_name=excluded.company_name,
			last_seen_at=excluded.last_seen_at,
			missing_from_source_at=NULL,
			updated_at=excluded.updated_at
	`), ev.Source, ev.Symbol, ev.CompanyName, ev.DelistedDate, seenAt, seenAt, seenAt)
	return err
}

func (t *delistingTx) MarkMissing(
	source string, present map[DelistingEventKey]struct{}, at time.Time,
) (int64, error) {
	existing, err := t.LoadEvents(source)
	if err != nil {
		return 0, err
	}
	var firstAbsent int64
	for _, ev := range existing {
		if _, ok := present[ev.Key()]; ok {
			continue
		}
		// `WHERE missing_from_source_at IS NULL` 讓這句天然冪等：
		// 持續缺席的列不會被更新，affected rows 因此只算「首次缺席」的轉換。
		res, err := t.tx.ExecContext(t.ctx, t.tx.Rebind(`
			UPDATE delisting_events
			SET missing_from_source_at = ?, updated_at = ?
			WHERE id = ? AND missing_from_source_at IS NULL
		`), at, at, ev.ID)
		if err != nil {
			return 0, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		firstAbsent += n
	}
	return firstAbsent, nil
}

const symbolProjectionColumns = `symbol, name, market, security_type, listed_date,
	is_listed, delisted_date, delisted_event_id`

func (t *delistingTx) SymbolStates(symbols []string) (map[string]SymbolProjectionState, error) {
	out := map[string]SymbolProjectionState{}
	if len(symbols) == 0 {
		return out, nil
	}
	// sqlx.In 展開後仍要 Rebind——展開出來的是 `?`，postgres 要的是 `$n`。
	query, args, err := sqlx.In(`SELECT `+symbolProjectionColumns+`
		FROM stock_symbols WHERE symbol IN (?)`, symbols)
	if err != nil {
		return nil, err
	}
	var rows []SymbolProjectionState
	if err := t.tx.SelectContext(t.ctx, &rows, t.tx.Rebind(query), args...); err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.Symbol] = row
	}
	return out, nil
}

func (t *delistingTx) DelistedListedStocks() ([]SymbolProjectionState, error) {
	var rows []SymbolProjectionState
	err := t.tx.SelectContext(t.ctx, &rows, t.tx.Rebind(`
		SELECT `+symbolProjectionColumns+`
		FROM stock_symbols
		WHERE is_listed = ? AND market LIKE ? AND security_type = ?
	`), false, "上市%", "股票")
	return rows, err
}

// casPredicate 組出投影／撤銷共用的 CAS 條件。
//
// 三組條件缺一不可：
//
//	① is_listed = false
//	② ownership 狀態仍等於讀取當時（空值 → 兩欄仍 NULL；job-owned → id 與 date 未變）
//	③ name / market / security_type / listed_date 與讀取時相同
//
// ⛔ **③ 不能省**：那四個欄位都是 candidate 的輸入，讀完之後被改掉的話，
// 記憶體裡的仲裁結果就已經過期，寫下去等於用舊資料做決定。
//
// ⛔ **nullable 欄位必須 null-safe**：listed_date / delisted_date / delisted_event_id
// 三個都可為 NULL，而 `x = NULL` 永遠不為 true。直接組出來的話**沒有任何競爭
// 也會誤判成落空**，那些列就永遠寫不進去也撤銷不掉。這裡依 snapshot 是否為 NULL
// 組出 `IS NULL` 或 `= ?`，三 engine 共用同一條產生邏輯
// （⛔ 不用 engine 專屬的 IS NOT DISTINCT FROM / <=>）。
//
// ⛔ **MySQL 的字串比較預設不是精確比較**：stock_symbols 沒有指定 collation，
// 繼承的 database 預設可能是 case-insensitive（MySQL 8 常見的
// utf8mb4_0900_ai_ci 即是）。那樣的話 `ABC-KY` 被改成 `abc-KY` 時
// Go 的正規化結果已經不同，SQL 卻仍判為相等，CAS 不會落空——
// 於是拿過期的仲裁結果寫入。所以 MySQL 一律寫成 `BINARY col = BINARY ?`。
// PostgreSQL 與 SQLite 的 `=` 本來就精確，不需處理。
func (t *delistingTx) casPredicate(prior SymbolProjectionState) (string, []any) {
	str := func(col string) string {
		if t.driver == "mysql" {
			return "BINARY " + col + " = BINARY ?"
		}
		return col + " = ?"
	}
	nullable := func(col string, valid bool) string {
		if !valid {
			return col + " IS NULL"
		}
		return col + " = ?"
	}

	conds := []string{
		"symbol = ?",
		"is_listed = ?",
		str("name"),
		str("market"),
		str("security_type"),
		nullable("listed_date", prior.ListedDate.Valid),
		nullable("delisted_date", prior.DelistedDate.Valid),
		nullable("delisted_event_id", prior.DelistedEventID.Valid),
	}
	args := []any{prior.Symbol, false, prior.Name, prior.Market, prior.SecurityType}
	if prior.ListedDate.Valid {
		args = append(args, prior.ListedDate.Time)
	}
	if prior.DelistedDate.Valid {
		args = append(args, prior.DelistedDate.Time)
	}
	if prior.DelistedEventID.Valid {
		args = append(args, prior.DelistedEventID.Int64)
	}
	return strings.Join(conds, " AND "), args
}

func (t *delistingTx) Project(
	prior SymbolProjectionState, date time.Time, eventID uint64, now time.Time,
) (bool, error) {
	where, args := t.casPredicate(prior)
	query := `UPDATE stock_symbols SET delisted_date = ?, delisted_event_id = ?, updated_at = ? WHERE ` + where
	full := append([]any{date, eventID, now}, args...)
	res, err := t.tx.ExecContext(t.ctx, t.tx.Rebind(query), full...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	// 目標值與現值不同時才走這條，所以 0 列**確實**代表 CAS 落空。
	return n > 0, nil
}

func (t *delistingTx) ConfirmProjection(prior SymbolProjectionState) (bool, error) {
	where, args := t.casPredicate(prior)
	query := `SELECT 1 FROM stock_symbols WHERE ` + where
	// ⛔ 必須是 current／locking read。MySQL 預設 REPEATABLE READ，本專案沒有覆寫
	// isolation，交易內的一般 SELECT 走 consistent snapshot——**看不到另一條連線
	// 已提交的變更**，最後仍會錯計 P。
	//
	// SQLite 沒有 FOR UPDATE，靠的是**單 writer 契約**：本 job 的三項寫入都在同一個
	// write transaction 內，該交易已持有 write lock。這是等價策略，不是漏寫。
	if t.driver != "sqlite" {
		query += " FOR UPDATE"
	}
	var one int
	err := t.tx.GetContext(t.ctx, &one, t.tx.Rebind(query), args...)
	if err == sql.ErrNoRows {
		return false, nil // 狀態已被改動 → X
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (t *delistingTx) Revoke(prior SymbolProjectionState, now time.Time) (bool, error) {
	where, args := t.casPredicate(prior)
	query := `UPDATE stock_symbols SET delisted_date = NULL, delisted_event_id = NULL, updated_at = ? WHERE ` + where
	full := append([]any{now}, args...)
	res, err := t.tx.ExecContext(t.ctx, t.tx.Rebind(query), full...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
