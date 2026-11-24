// Package bouncereplay 是 I-140 的 A／B 離線重播（docs/plans/i140-intraday-bounce-dedup.md 第 7.3 節）。
//
// ⛔ 只在 dev 使用：⛔ 不掛進 scheduler、⛔ 不在 production 的執行路徑上（Dockerfile 只 build cmd/server）。
// 只連隔離的 dev PostgreSQL（cmd/bouncereplay 檢查資料庫名稱）。
//
// 流程：依 live 實際評估的（輪次、檔）、照 eval_at 的順序，把 K 棒放到那一輪的最後一根、時鐘設成 eval_at，
// 用同一個 signal.Engine 分別跑 A（價位鍵）與 B（同一檔 15 分鐘），A 的結果再跟 live 逐筆對帳、分群。
package bouncereplay

import (
	"time"

	"github.com/trading/backend/internal/store"
)

const (
	// Timeframe 是盤中評估的時間框架。
	Timeframe = "1m"
	// Cooldown 是兩種規則共用的冷卻（跟 signal 套件的 signalCooldown 相同）。
	Cooldown = 15 * time.Minute
	// StaleAfter 跟 scheduler.IntradayStaleAfter 相同：data_at − last_bar_ts 超過它就 Withhold。
	StaleAfter = 10 * time.Minute
	// RecentLimit 是 A 的 DB 判重讀的最近筆數（signal 套件的 recentSignalLimit）。
	RecentLimit = 20
	// Lookback 是評估實際會讀的 K 棒根數（indicator.Lookback）。
	Lookback = 120
	// BoundaryWindow 是時鐘邊界的篩選寬度（只用來篩選，見 explainClockBoundary）。
	BoundaryWindow = time.Second
	// lockMargin 是「前 15 分鐘內的 live 送出」的保守邊界（live 的保留時刻與 created_at 相差毫秒級）。
	lockMargin = 2 * time.Second
)

// Eval 是 live 一次實際的盤中評估（intraday_symbol_runs 的一列，eval_status ⛔ 不是 not_evaluated）。
type Eval struct {
	RunID       uint64
	Symbol      string
	DataAt      time.Time
	EvalAt      time.Time
	LastBarTS   time.Time
	Fingerprint string
	EvalStatus  string
	EvalCause   string
}

func (e Eval) key() evalKey { return evalKey{e.RunID, e.Symbol} }

type evalKey struct {
	runID  uint64
	symbol string
}

// LiveSignal 是 live signals 的一列。
type LiveSignal struct {
	ID         uint64           `db:"id"`
	Symbol     string           `db:"symbol"`
	SignalType string           `db:"signal_type"`
	Direction  string           `db:"direction"`
	Price      float64          `db:"price"`
	Volume     int64            `db:"volume"`
	VolRatio   float64          `db:"vol_ratio"`
	Resistance float64          `db:"resistance"`
	Support    float64          `db:"support"`
	Trend      string           `db:"trend"`
	Note       string           `db:"note"`
	Strength   float64          `db:"strength"`
	ChipSignal store.NullString `db:"chip_signal"`
	TS         time.Time        `db:"ts"`
	CreatedAt  time.Time        `db:"created_at"`
}

// Input 是重播的全部輸入（由 Load 從隔離的來源庫讀出，或測試自己造）。
type Input struct {
	From, To time.Time // 窗口：data_at ∈ [From, To)
	Evals    []Eval    // 依 eval_at 排序
	Candles  map[string][]store.Candle
	Signals  []LiveSignal // 依 created_at 排序；含窗口之前的起始歷史
	// ExpectedDays 是交接的預定交易日清單（台北日期，YYYY-MM-DD）：預檢與判定都依它檢查完整性——
	// 清單上的每一天都要有評估、清單外的日期⛔ 不能出現，空清單或整個窗口沒有評估都判不通過（複審 2026-11-24）。
	ExpectedDays []string
}

// 分群（第 7.3 節「對帳與覆蓋率」）。
const (
	GroupReconcilable = "reconcilable"
	GroupInputChanged = "input_changed"
	GroupDiverged     = "history_diverged"
	GroupUnconfirmed  = "history_unconfirmed"
)

// A 對 live 的對帳結果（只有可對帳的那一群有值）。
const (
	MatchOK            = "match"
	MatchLiveAbnormal  = "live_abnormal"  // live 的 evaluate_failed／degraded：⛔ 不算進一致率、留在分母
	MatchClockBoundary = "clock_boundary" // 已解釋的不一致（只限 A 對 live）
	MatchUnexplained   = "unexplained"    // 原因不明 → 停下
)

// Outcome 是一次重播評估的結果。
type Outcome struct {
	Status       string // 跟 scheduler 寫進 eval_status 的對應相同
	Err          string
	SuppressedBy string
	Candidate    *store.Signal
	Sent         *store.Signal
	Key          string // A 的身分鍵（保留鎖的鍵）
}

// Record 是一次評估的完整紀錄（輸出 CSV 的一列）。
type Record struct {
	Eval
	Day         string
	Withhold    bool
	Group       string
	GroupReason string
	A, B        Outcome
	AMatch      string
	MatchDetail string
	// BAnchorSupport：B 抑制、A 送出的候選，B 當時的錨點支撐（第 7.3 節衡量口徑 3）。
	BAnchorSupport *float64
	// LiveSent 是這一列在 live 寫進 signals 的那一筆（唯一配對；沒有就是 nil）。
	LiveSent *LiveSignal
	// NonTargetDiff：非目標訊號（⛔ 不是 SUPPORT_BOUNCE）在 A、B 的結果不同（第 7.3 節衡量口徑 4）。
	// 只有逐筆證實來自「最近 20 筆」的邊界才算已解釋（NonTargetExplained）。
	NonTargetDiff      string
	NonTargetExplained bool
}

// counted：可對帳而且 A 跟 live 一致——覆蓋率的分子與「只算可對帳」的比較範圍。
// 時鐘邊界（已解釋）與 live 評估異常⛔ 不算進分子，只留在分母（複審 2026-11-24）。
func (r Record) counted() bool { return r.Group == GroupReconcilable && r.AMatch == MatchOK }
