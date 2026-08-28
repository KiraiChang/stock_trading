package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// 日 K 缺漏偵測的驗證結果值域（見 `docs/database-schema.md`；原記於 issue.md I-091，已收斂）。
//
// **四個值不是同一個軸上的嚴重度階梯**，雖然 coalesce 時會照嚴重度取最大：
// `gap` 是「驗成功了，結論是有缺口」，`unavailable` 是「根本沒驗成」。
// 把兩者混為一談會讓「驗不了」被記成「驗過了沒問題」——那是這整個機制最該避免的失效。
const (
	// VerificationVerified 驗過，沒有缺口。
	VerificationVerified = "verified"
	// VerificationGap 驗過，確認有缺口。**這是成功的驗證**，只是結論是壞消息。
	VerificationGap = "gap"
	// VerificationDeferred 對照來源還沒發布到那一天：既不算缺口也不算失敗。
	VerificationDeferred = "deferred"
	// VerificationUnavailable 驗不了：請求失敗、格式變動，或該來源根本不提供這種查詢。
	VerificationUnavailable = "unavailable"
)

// CandleVerificationState 是某個 (symbol, timeframe) 的驗證簿記。
//
// **時間欄位可為 NULL**，語意各不相同：
//   - LastAttemptedAt 為 NULL：有列但從未嘗試過（實務上不會出現，因為列是在第一次
//     attempt 時才寫入）。
//   - LastVerifiedAt 為 NULL：嘗試過但從未成功驗證。
//
// ⚠️ **「沒有列」與「有列但欄位為 NULL」是兩件事**：首次出現的候選根本沒有列，
// LoadStates 不會回傳它。公平排序把「沒有列」排在最前面，那個判斷只能在 Go 端做
// （見 LoadStates 的說明）。
type CandleVerificationState struct {
	Symbol              string   `db:"symbol"`
	Timeframe           string   `db:"timeframe"`
	LastAttemptedAt     NullTime `db:"last_attempted_at"`
	LastVerifiedAt      NullTime `db:"last_verified_at"`
	LastResult          string   `db:"last_result"`
	ConsecutiveFailures int      `db:"consecutive_failures"`
}

// VerificationAttempt 是**本輪這個 symbol 的整體結論**，不是單次請求的結果。
//
// ⚠️ **同一個 symbol 在一輪裡可能被驗多次**（視窗跨月時同一檔要查兩個月份，
// 也可能同時落在多個 aggregate 分組）。呼叫端必須**先把跨月份／跨日期的結果彙整成
// 唯一一筆**再送進來，合併規則見 `docs/database-schema.md` 的 `candle_verification_state`：
//
//	LastAttemptedAt      本輪最後一次 attempt 的時間
//	LastVerifiedAt       只要有任何一次成功驗證（verified 或 gap）就更新
//	LastResult           取最嚴重：unavailable > gap > deferred > verified
//	ConsecutiveFailures  有任何成功 → 歸零；沒有任何成功且至少一個 unavailable → +1；
//	                     其餘（只有 deferred）不動
//
// **不是在 repo 前機械式挑一個最嚴重的字串**——四個欄位各有各的規則，
// 只合併 LastResult 會讓「一個月份成功、一個月份失敗」的進度被記成沒驗過。
type VerificationAttempt struct {
	Symbol          string
	Timeframe       string
	LastAttemptedAt time.Time
	// LastVerifiedAt 為零值代表本輪沒有任何一次成功驗證——此時**保留資料庫既有的值**，
	// 不覆寫成 NULL。否則一次失敗就會把先前的驗證進度抹掉。
	LastVerifiedAt      time.Time
	LastResult          string
	ConsecutiveFailures int
}

// CandleVerificationRepo 是缺漏偵測的驗證簿記存取層。
//
// **刻意只有兩支方法，而且都不做排序**：排序與缺席合併是 Go 的事，理由見 LoadStates。
type CandleVerificationRepo interface {
	// LoadStates 回傳這批 symbol 中**已經有 state 的全部列**，鍵是 symbol。
	//
	// ⚠️ **刻意沒有 limit，也刻意不排序。**
	//
	// 帶 limit 會與「Go 端合併排序」直接矛盾：repo 若先截斷，**沒被回傳的既有 state
	// 會被呼叫端誤認成「從未出現」而排到最前面**——公平排序直接壞掉，而且壞得很安靜。
	//
	// 不排序是因為排序鍵的第一順位是「有沒有 state」，而**沒有 state 的候選根本不在
	// 查詢結果裡**。`NULLS FIRST` 解不了這個問題（它處理的是 NULL 欄位不是缺列），
	// 而且 MySQL 不支援、repo 至今從未用過。所以候選清單與這份 map 的 LEFT-merge
	// 一律在 Go 端做。
	//
	// symbols 為空時回空 map，不送查詢。
	LoadStates(ctx context.Context, timeframe string, symbols []string) (map[string]CandleVerificationState, error)

	// RecordAttempts 批次寫入本輪的驗證結論（upsert）。
	//
	// ⚠️ **批次內不得有重複的 (symbol, timeframe)**。PostgreSQL 的
	// `INSERT … ON CONFLICT DO UPDATE` 不允許同一個 statement 更新同一列兩次
	// （`ON CONFLICT DO UPDATE command cannot affect row a second time`），
	// 那會直接報錯而讓**整批**寫入失敗。
	//
	// 去重是呼叫端的責任（它要做的本來就是「彙整成這個 symbol 的整體結論」），
	// 但這裡仍會擋下重複鍵並回傳可讀的錯誤——讓它變成一句說得清楚的話，
	// 而不是一個要查文件才看得懂的 postgres 錯誤碼。
	RecordAttempts(ctx context.Context, attempts []VerificationAttempt) error
}

type candleVerificationRepo struct {
	db     *sqlx.DB
	driver string
}

func NewCandleVerificationRepo(db *sqlx.DB) CandleVerificationRepo {
	return &candleVerificationRepo{db: db, driver: db.DriverName()}
}

func (r *candleVerificationRepo) LoadStates(
	ctx context.Context, timeframe string, symbols []string,
) (map[string]CandleVerificationState, error) {
	if len(symbols) == 0 {
		return map[string]CandleVerificationState{}, nil
	}
	// sqlx.In 展開 IN 的佔位符後仍要 Rebind——展開出來的是 `?`，postgres 要的是 `$n`
	// （與 CandleRepo.SymbolsWithCandleOn 同一個寫法）。
	query, args, err := sqlx.In(`
		SELECT symbol, timeframe, last_attempted_at, last_verified_at,
		       last_result, consecutive_failures
		FROM candle_verification_state
		WHERE timeframe = ? AND symbol IN (?)
	`, timeframe, symbols)
	if err != nil {
		return nil, err
	}
	var rows []CandleVerificationState
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, err
	}
	out := make(map[string]CandleVerificationState, len(rows))
	for _, row := range rows {
		out[row.Symbol] = row
	}
	return out, nil
}

func (r *candleVerificationRepo) RecordAttempts(
	ctx context.Context, attempts []VerificationAttempt,
) error {
	if len(attempts) == 0 {
		return nil
	}
	// 先擋重複鍵：postgres 會在整批 upsert 時直接報錯，訊息看不出是哪個 symbol。
	seen := make(map[string]struct{}, len(attempts))
	for _, a := range attempts {
		key := a.Symbol + "\x00" + a.Timeframe
		if _, dup := seen[key]; dup {
			return fmt.Errorf(
				"candle verification attempts 含重複鍵 (symbol=%s, timeframe=%s)；"+
					"呼叫端須先把該 symbol 跨月份／日期的結果彙整成唯一一筆",
				a.Symbol, a.Timeframe)
		}
		seen[key] = struct{}{}
	}

	rows := make([]map[string]any, 0, len(attempts))
	for _, a := range attempts {
		var verified NullTime
		if !a.LastVerifiedAt.IsZero() {
			verified.Time = a.LastVerifiedAt
			verified.Valid = true
		}
		rows = append(rows, map[string]any{
			"symbol":               a.Symbol,
			"timeframe":            a.Timeframe,
			"last_attempted_at":    a.LastAttemptedAt,
			"last_verified_at":     verified,
			"last_result":          a.LastResult,
			"consecutive_failures": a.ConsecutiveFailures,
		})
	}
	_, err := r.db.NamedExecContext(ctx, r.attemptUpsertSQL(), rows)
	return err
}

// attemptUpsertSQL 的 last_verified_at 一律用 COALESCE 保護既有值。
//
// **本輪沒有任何成功驗證時傳 NULL 進來，不能覆寫掉先前的成功時間**——那會讓
// 「上次驗成功是什麼時候」在一次失敗之後永遠遺失，公平排序與陳舊判斷都會失準。
// 其餘欄位是本輪的結論，直接覆寫。
func (r *candleVerificationRepo) attemptUpsertSQL() string {
	const cols = `(symbol, timeframe, last_attempted_at, last_verified_at,
		last_result, consecutive_failures)
		VALUES (:symbol, :timeframe, :last_attempted_at, :last_verified_at,
		:last_result, :consecutive_failures)`
	switch r.driver {
	case "mysql":
		return `INSERT INTO candle_verification_state ` + cols + `
			ON DUPLICATE KEY UPDATE
				last_attempted_at=VALUES(last_attempted_at),
				last_verified_at=COALESCE(VALUES(last_verified_at), last_verified_at),
				last_result=VALUES(last_result),
				consecutive_failures=VALUES(consecutive_failures)`
	}
	// postgres 與 sqlite 共用 ON CONFLICT 語法。
	return `INSERT INTO candle_verification_state ` + cols + `
		ON CONFLICT(symbol, timeframe) DO UPDATE SET
			last_attempted_at=excluded.last_attempted_at,
			last_verified_at=COALESCE(excluded.last_verified_at, candle_verification_state.last_verified_at),
			last_result=excluded.last_result,
			consecutive_failures=excluded.consecutive_failures`
}
