package store

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// sqliteBusyTimeoutMS 是拿不到 writer lock 時的等待上限（毫秒）。
//
// **這個值是算出來的，不是拍的**：本專案最長的單一 SQLite 交易（delisting_reconcile
// 的事件 upsert + 主檔投影 + 快照）正常在毫秒級完成，5 秒足以吸收一次**外部 writer**
// 的短暫持鎖，又遠短於任何 job 自己的 timeout。
//
// ⚠️ **它管的是「外部 writer」，不是同程序的排程重疊**：正式程式只建立一個 pool
// （cmd/server/main.go 的 store.NewDB），SQLite 的 pool 又限制成一條 connection，
// 所以同程序的兩支 job 會先在 database/sql 的 pool 排隊，根本不會進到 SQLite 層
// 拿 SQLITE_BUSY。真正會競爭 writer lock 的是另一個程序或第二個 handle。
const sqliteBusyTimeoutMS = "5000"

// enforcedSQLitePragmas 是**不可被 DSN 覆寫**的連線設定。
//
// 兩者都是 **connection-local**：pool 重建 physical connection 時，新 connection
// 不會繼承先前用 `db.Exec("PRAGMA ...")` 設過的值，設定會**靜默失效**。
// 所以它們必須走 DSN——driver 對每一條新 connection 都會套用。
//
// ⛔ **foreign_keys 尤其不能漏**：SQLite 的 FK 預設是關的，一旦失效，
// stock_symbols.delisted_event_id 的 RESTRICT 與 delisting_source_snapshots.job_run_id
// 的 SET NULL 全部落空，而且**不會有任何東西報錯**。
//
// ℹ️ `journal_mode=WAL` 不在這裡：它是寫進 DB 檔的**持久設定**，不是 connection-local，
// 語意與這兩個不同，仍留在 NewSQLite 裡以 Exec 設定一次即可。
var enforcedSQLitePragmas = []string{
	"busy_timeout(" + sqliteBusyTimeoutMS + ")",
	"foreign_keys(1)",
}

// buildSQLiteDSN 把使用者給的 DSN 改寫成帶強制 pragma 的 file: URI。
//
// **為什麼不能字串拼接 `?_pragma=...`**：DSN 有六種形態，拼接會做出第二個 `?`，
// 或蓋掉既有的 mode / _txlock / cache 等參數。
//
//	file-backed：①空字串（回退 ./trading.db）②純檔名／相對路徑（live 現況）
//	             ③已帶 query 的 file: URI
//	memory：     ④裸 :memory: ⑤file::memory:?cache=shared
//	             ⑥named memory URI：file:<name>?mode=memory&cache=shared
//
// **「要不要 filepath.Abs」是三分支**：
//
//	裸 :memory:        → 特判成 url.URL{Scheme:"file", Opaque:":memory:"}
//	任何 file: URI      → 一律保留 URI 語意，不做 Abs、不改 path/opaque，只合併 query
//	一般檔案路徑        → filepath.Abs 後組 file:// URI
//
// ⛔ **相對路徑一定要先 Abs**：url.URL 在 Host 為空但 Path 不以 `/` 開頭時仍會輸出
// `//`，於是 `file://../../output/x.db` 的 `..` 會被當成 authority，路徑就被改掉了。
// live 的 DSN 正是相對路徑（config.yaml 的 ../../output/stock_trading/trading.db），
// 這條路一定會被踩到。⚠️ Abs 以行程的工作目錄為基準，與「直接把相對路徑交給 driver」
// 的既有行為一致。
//
// ⛔ **裸 :memory: 不能走 Abs**：filepath.Abs(":memory:") 會得到 <cwd>/:memory:，
// 把記憶體 DB 靜默變成磁碟檔。
func buildSQLiteDSN(dsn string) (string, error) {
	if dsn == "" {
		dsn = "./trading.db"
	}

	var u *url.URL
	switch {
	case dsn == ":memory:":
		u = &url.URL{Scheme: "file", Opaque: ":memory:"}
	case strings.HasPrefix(dsn, "file:"):
		parsed, err := url.Parse(dsn)
		if err != nil {
			return "", err
		}
		u = parsed
	default:
		abs, err := filepath.Abs(dsn)
		if err != nil {
			return "", err
		}
		// url.URL 只編碼 ? / # / 空白等會破壞 URI 的字元，`/` 保持原樣。
		// ⛔ 不可對整條路徑做 percent-encode，那會把目錄分隔的 `/` 編成 %2F。
		u = &url.URL{Scheme: "file", Path: abs}
	}

	q := u.Query()
	kept := make([]string, 0, len(q["_pragma"])+len(enforcedSQLitePragmas))
	for _, p := range q["_pragma"] {
		if isEnforcedSQLitePragma(p) {
			continue
		}
		kept = append(kept, p)
	}
	kept = append(kept, enforcedSQLitePragmas...)
	q["_pragma"] = kept
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// isEnforcedSQLitePragma 判斷一個 `_pragma` 值是不是被本專案接管的那兩個。
//
// ⛔ **不能只看 `(`**：driver 也吃 `foreign_keys=OFF` 這種等號寫法，
// 只依 `(` 切名稱的話它會整個穿過去，強制政策等於沒做。要涵蓋的變體：
//
//	括號        foreign_keys(0)
//	等號        foreign_keys=OFF、busy_timeout=0
//	空白        foreign_keys = 0、" foreign_keys(0) "
//	大小寫      FOREIGN_KEYS(0)、Busy_Timeout=0
//	URL encode  foreign_keys%3D0（由 url.Query() decode 後即等號形式）
//
// ⛔ **不要在這裡再 decode 一次**：呼叫端傳進來的是 url.Values 的值，
// **已經 decode 過**。再 decode 會把 `foreign_keys%253D0` 展開成 `foreign_keys=0`，
// 讓一個本來不該被移除的值被誤判成衝突項。decode 只發生在 parser 那一次。
func isEnforcedSQLitePragma(pragma string) bool {
	name := pragma
	if i := strings.IndexAny(name, "(="); i >= 0 {
		name = name[:i]
	}
	// TrimSpace 放在切完之後就夠：前導空白不含 `(` / `=`，切點不受影響，
	// 而尾隨空白（` foreign_keys (0)` 這種）只有切完才去得掉。
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "busy_timeout" || name == "foreign_keys"
}

func NewSQLite(dsn string) (*sqlx.DB, error) {
	connDSN, err := buildSQLiteDSN(dsn)
	if err != nil {
		return nil, err
	}
	db, err := sqlx.Connect("sqlite", connDSN)
	if err != nil {
		return nil, err
	}
	// WAL 模式提升並行讀取效能。
	// ℹ️ 這一個**可以**用 Exec 設：journal_mode 是寫進 DB 檔的持久設定，
	// 不是 connection-local，重建 connection 不會失效。
	// busy_timeout / foreign_keys 則相反，已改由 DSN 帶（見 enforcedSQLitePragmas）。
	db.MustExec(`PRAGMA journal_mode=WAL`)
	// SQLite 建議單一 writer，讀取可多個
	db.SetMaxOpenConns(1)
	return db, nil
}
