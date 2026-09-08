package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/joberr"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// 終止上市名單對帳排程（計畫書 docs/todo.md T-071）。
//
// ⚠️ **這個 job 不決定 is_listed**——那是 stock_symbol_sync 的權責。
// 它只補官方的終止上市日期，並在主檔與官方名單對不上時告警。

// delistingReconcileJobName 是 job_runs.job_name 的值（19 字元）。
// ℹ️ job_name 已是 VARCHAR(64)（migration 063），不需要再放寬。
const delistingReconcileJobName = "delisting_reconcile"

// delistingReconcileTimeout 是整輪的上限：單一 HTTP 請求（實測 8 KB）＋ 一個交易。
// 比 stock_symbol_sync 的 20 分鐘寬鬆得多，5 分鐘已是數量級的餘裕。
const delistingReconcileTimeout = 5 * time.Minute

// SetDelistingReconcile 注入這個 job 的依賴。未注入或未啟用時不註冊排程，
// 行為與導入前完全相同（比照 adjuster / evaluationUniverse / srAnalysis）。
func (s *Scheduler) SetDelistingReconcile(rec *market.DelistingReconciler, cfg config.DelistingConfig) {
	s.delistingReconciler = rec
	s.delistingCfg = cfg
}

// defaultDelistingReconcileCron：stock_symbol_sync 是 06:30，本 job 排在它之後。
// ⚠️ **時間只是常態安排**：正確性由「當日 sync 必須已成功」那條依賴保證，
// 不是靠時間錯開——sync 可能失敗、被停用、逾時或與手動觸發重疊。
const defaultDelistingReconcileCron = "0 7 * * *"

// delistingCron 是實際註冊的 cron 字串。config 沒設不該讓這支排程消失
// （比照 corporateActionCron）。
func (s *Scheduler) delistingCron() string {
	if s.delistingCfg.Cron != "" {
		return s.delistingCfg.Cron
	}
	return defaultDelistingReconcileCron
}

// ── single-flight：三個方法職責封閉（比照 SR analysis pattern）─────────────
//
// ⛔ **單一 transaction 只保證「單輪完整」，不保證「兩輪的先後」**：
// 兩輪若抓到不同版本的 CSV，較舊的一輪可能較晚 commit，反過來把新事件標記成消失，
// 而且兩輪各自看起來都成功。

// acquireDelistingReconcile 取得執行所有權，回傳是否成功。
func (s *Scheduler) acquireDelistingReconcile() bool {
	return s.delistingRunning.CompareAndSwap(false, true)
}

func (s *Scheduler) releaseDelistingReconcile() { s.delistingRunning.Store(false) }

// RunDelistingReconcile 是 **cron 的入口**：自己取得所有權、自己釋放。
//
// ⛔ **刻意不收 acceptShrink 參數**——「cron 結構上不可能取得 override」這個保證
// 必須由**型別**兌現，而不是靠呼叫端記得傳 false。留著參數就留著一個誤傳的入口，
// 而那個 override 會靜默放行一次縮水。
//
// ⛔ **取不到鎖時只記 Warn、不寫 job_run**。SR pattern 在這裡**不適用**：
// 它的兩支 cron 是兩個不同的 job_name，彼此的 latest-run 投影互不干擾；
// 本 job 的 cron 與手動**共用同一個 job_name**。寫一筆 failed 的話，
// 「手動先開始 → cron 撞上寫 failed → 手動接著成功」會讓 GetLatestPerJob
// （ORDER BY started_at DESC, id DESC）選到 cron 那筆 failed，
// 把一次成功的執行顯示成失敗。要保存觸發失敗得另設 attempt 類型，本筆不做。
func (s *Scheduler) RunDelistingReconcile() {
	if !s.acquireDelistingReconcile() {
		s.log.Warn("delisting reconcile 已在執行中，本輪跳過（不寫 job_run，避免蓋掉進行中那輪的結果）")
		return
	}
	defer s.releaseDelistingReconcile()
	s.runDelistingReconcileOwned(context.Background(), false)
}

// TryStartDelistingReconcile 是 **API 手動觸發的入口**：同步取得所有權，成功才把工作丟到背景。
//
// 回傳 false 時呼叫端應回 **409**。
// ⛔ **不要先回 202 再由背景 goroutine 靜默跳過**——既有的 evaluation_universe_sync
// 正是後者（handler 先回 202、scheduler 才在背景 CompareAndSwap 失敗後記一行 Warn），
// 呼叫端會以為觸發成功、實際上什麼都沒發生。本 job 不沿用那個模式。
//
// **釋放由這裡 spawn 的 goroutine 負責**，呼叫端不需要也不應該碰。
func (s *Scheduler) TryStartDelistingReconcile(acceptShrink bool) bool {
	if !s.acquireDelistingReconcile() {
		return false
	}
	go func() {
		defer s.releaseDelistingReconcile()
		s.runDelistingReconcileOwned(context.Background(), acceptShrink)
	}()
	return true
}

// runDelistingReconcileOwned 是核心：⛔ **不再取鎖**，鎖由入口的 defer 負責。
//
// ⚠️ **早退路徑也不得自行釋放鎖**——抓取失敗、解析失敗、縮水防護、依賴不成立
// 都在這裡 return，全部靠入口的 defer。
func (s *Scheduler) runDelistingReconcileOwned(ctx context.Context, acceptShrink bool) {
	if s.delistingReconciler == nil {
		return
	}

	runID := s.startRun(ctx, delistingReconcileJobName)
	// ⛔ **startRun 失敗（runID = 0）時必須中止。**
	// 那代表這一輪**沒有任何 audit record**——繼續做下去會寫入一整輪資料，
	// 卻在 job_runs 上完全看不到，事後查不到是誰改的。
	// ⚠️ 這條不只影響 SQLite，任何 engine 的 startRun 失敗都適用。
	if runID == 0 {
		s.log.Error("delisting reconcile 取不到 job_run id，本輪中止（不做任何業務寫入）")
		return
	}

	// 只有同步本身套 timeout；finishRunStatus 走 context.WithoutCancel，
	// 所以逾時之後仍寫得回 job_runs（不會永遠卡在 running）。
	runCtx, cancel := context.WithTimeout(ctx, delistingReconcileTimeout)
	defer cancel()

	now := time.Now().In(timeutil.TaipeiTZ)
	out, err := s.delistingReconciler.Reconcile(runCtx, runID, acceptShrink, now)
	if err != nil {
		// ⚠️ symbols_failed 恆為 0：這個 job 沒有「逐檔失敗」的概念，
		// 而那兩個欄位的單位是**標的數**。⛔ 不得用假的 1/1 去湊。
		s.log.Error("delisting reconcile failed",
			zap.Bool("accept_shrink", acceptShrink), zap.Error(err))
		s.finishRunStatus(ctx, runID, delistingReconcileJobName, "failed",
			out.DistinctSymbols, 0, safeJobErrorSummary("delisting_reconcile", err))
		return
	}

	// 完整計數只進結構化 log——job_runs 沒有自由欄位。
	s.log.Info("delisting reconcile completed",
		append(out.Fields(), zap.Bool("accept_shrink", acceptShrink))...)

	// ⚠️ partial 也必須寫得出原因，否則排程頁只看得到 partial、看不到為什麼。
	// Summary 只含類別名與數字（沒有主機、DSN 或 SQL 片段），
	// 用 SafeMessenger 讓它原樣通過而不是被壓成 internal_error。
	var reason string
	if out.Degraded() {
		reason = joberr.Describe(delistingDegradedError{summary: out.Summary()})
	}
	s.finishRunDegraded(ctx, runID, delistingReconcileJobName,
		out.DistinctSymbols, 0, reason, out.Degraded())
}

// delistingDegradedError 讓 partial 的原因原樣寫進 job_runs.error。
//
// 訊息由 DelistingOutcome.Summary() 完整組出，只有類別名與數字，沒有主機、DSN、
// SQL 片段或任何外來字串，所以符合 joberr.SafeMessenger 的兩個條件。
// ⛔ **不能改用 joberr.Summary**——它走的是 Classify，會把整段計數壓成
// `internal_error`，那是資訊淨損失、零安全收益（排程頁只看得到 partial、
// 看不到為什麼）。
type delistingDegradedError struct{ summary string }

func (e delistingDegradedError) Error() string { return e.SafeJobMessage() }

func (e delistingDegradedError) SafeJobMessage() string { return e.summary }

// ── 當日 stock_symbol_sync 是否成功 ───────────────────────────────────────

// JobRunReadiness 用 job_runs 判斷「當日的 stock_symbol_sync 成功了嗎」。
//
// ⚠️ 這是 fail-closed 的前置：sync 沒跑成功時 is_listed 的狀態是舊的或不完整的，
// 拿它當投影前置沒有意義。只把 cron 排在它後面**不構成依賴**。
type JobRunReadiness struct {
	repo store.JobRunRepo
}

func NewJobRunReadiness(repo store.JobRunRepo) *JobRunReadiness {
	return &JobRunReadiness{repo: repo}
}

func (r *JobRunReadiness) StockSymbolSyncSucceededToday(ctx context.Context, now time.Time) (bool, error) {
	rows, err := r.repo.GetLatestPerJob(ctx)
	if err != nil {
		return false, err
	}
	today := now.In(timeutil.TaipeiTZ).Format("2006-01-02")
	for _, row := range rows {
		if row.JobName != "stock_symbol_sync" {
			continue
		}
		// 要求**最新一筆**是 success 且起跑於台北時間今日。
		// ⚠️ 「今天跑過但失敗」與「今天還沒跑」處置相同——兩者的 is_listed 都不可信。
		return row.Status == "success" &&
			row.StartedAt.In(timeutil.TaipeiTZ).Format("2006-01-02") == today, nil
	}
	return false, nil
}
