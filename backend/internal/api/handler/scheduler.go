package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/store"
)

type SchedulerHandler struct {
	repo  store.JobRunRepo
	sched *scheduler.Scheduler
	log   *zap.Logger
}

func NewSchedulerHandler(repo store.JobRunRepo, sched *scheduler.Scheduler, log *zap.Logger) *SchedulerHandler {
	return &SchedulerHandler{repo: repo, sched: sched, log: log}
}

// POST /api/v1/scheduler/daily-close/run
// 手動重跑「收盤後拉日K + 完整掃描」，用於排程時間點 FinMind 當天日K
// 還沒發布（拉到 count=0）時的補救，跟 cron 觸發共用同一份邏輯
// （見 scheduler.Scheduler.RunDailyClose），在背景執行、立即回應。
func (h *SchedulerHandler) RunDailyClose(c *gin.Context) {
	go h.sched.RunDailyClose()
	c.JSON(http.StatusAccepted, gin.H{"message": "daily_close 已在背景重新觸發"})
}

// POST /api/v1/scheduler/stock-symbol-sync/run
// 手動重跑 TWSE ISIN 股票主檔同步，與每日 stock_symbol_sync cron 共用同一份邏輯。
func (h *SchedulerHandler) RunStockSymbolSync(c *gin.Context) {
	go h.sched.RunStockSymbolSync()
	c.JSON(http.StatusAccepted, gin.H{"message": "stock_symbol_sync 已在背景重新觸發"})
}

// POST /api/v1/scheduler/sr-evaluation/run
// 手動重跑 SR Zone evaluation / decision replay，與 sr_evaluation cron 共用同一份邏輯。
func (h *SchedulerHandler) RunSREvaluation(c *gin.Context) {
	go h.sched.RunSREvaluation()
	c.JSON(http.StatusAccepted, gin.H{"message": "sr_evaluation 已在背景重新觸發"})
}

// POST /api/v1/scheduler/corporate-action-sync/run
// 手動重跑公司行動同步（分割 ＋ 除權息）與還原係數重算，與每日 cron 共用同一份邏輯。
//
// **部署後的驗證需要它**：cron 是平日 06:30，若部署發生在那之後，
// 沒有這個入口就得等到隔天才驗得了還原是否正確（見 scripts/verify-adjustment.sh）。
// 重算是冪等的，重複觸發不會累積誤差。
func (h *SchedulerHandler) RunCorporateActionSync(c *gin.Context) {
	go h.sched.RunCorporateActionSync()
	c.JSON(http.StatusAccepted, gin.H{"message": "corporate_action_sync 已在背景重新觸發"})
}

// POST /api/v1/scheduler/evaluation-universe-sync/run
// 手動重跑評估標的池的日 K 維護，與 evaluation_universe_sync cron 共用同一份邏輯。
//
// **這個入口是必要的**：cron 預設關閉（一次約 26 分鐘、131 個 FinMind 請求），
// 而跑 evaluation 之前需要先把池的尾端對齊——在排程開啟之前，這是唯一的對齊方式。
// 重複觸發由 scheduler 內的旗標擋掉，不會有兩批請求互搶節流器。
func (h *SchedulerHandler) RunEvaluationUniverseSync(c *gin.Context) {
	go h.sched.RunEvaluationUniverseSync()
	c.JSON(http.StatusAccepted, gin.H{"message": "evaluation_universe_sync 已在背景重新觸發"})
}

// RunSRAnalysis 手動觸發 SR 分析排程（contract 見 docs/api-reference.md）。
//
// `with_chip=true` 走 22:00 那輪的規則（額外要求當日籌碼已入庫）。**這個入口是必要的**：
// cron 預設關閉，而 todo.md T-049 前置①（新舊兩套 active 事件集合的逐日並行比對）
// 要的母體得先有辦法補跑；另外排程漏跑時也只有這裡能補。
// **這裡原本還寫著 issue.md I-074，2026-09-01 移除**：decision replay 讀的是 candles
// （run_decision_replay -> _load_db_sources），從來不依賴這個排程。
//
// **兩個時段共用一個執行所有權**（不是各自一個旗標）：任一輪在跑時這裡回 409，
// 而不是讓兩輪並行——這台 host 只有 2GiB，逐檔的峰值等同使用者手動點一次分析。
// 取得所有權是**同步**的，所以 409 與 202 是可信的答案，不是猜的；
// 背景工作與釋放都由 scheduler.TryStartSRAnalysis 負責。
func (h *SchedulerHandler) RunSRAnalysis(c *gin.Context) {
	withChip := c.Query("with_chip") == "true"
	running, started := h.sched.TryStartSRAnalysis(withChip)
	if !started {
		c.JSON(http.StatusConflict, gin.H{
			"error":       "sr analysis already running",
			"running_job": running,
		})
		return
	}
	job := "sr_analysis"
	if withChip {
		job = "sr_analysis_chip"
	}
	c.JSON(http.StatusAccepted, gin.H{"message": job + " 已在背景觸發"})
}

// KnownSchedulerJobs 匯出給測試用：DB 的 job_name 欄位必須容得下每一個名稱
// （2026-08-11 正式環境因 VARCHAR(20) 裝不下 corporate_action_sync 而失敗）。
func KnownSchedulerJobs() []string { return append([]string(nil), knownSchedulerJobs...) }

// **`sr_zone_verify` 沒有自己的 cron**：它在 `RunDailyClose` 尾端無條件執行，
// 但寫的是獨立的 job_runs 紀錄（失敗不影響 daily_close 的判定）。
// 不列進來的話它的失敗只能靠直接查 DB 才看得到。
// **`candle_gap_detection` 同理**：它掛在 `runEvaluationUniverseSync` 尾端、沒有自己的
// cron，但寫獨立的 job_runs（現況見 `docs/architecture.md`）。它的註冊條件比 parent 嚴格——
// 自身 enabled ＋ 四項依賴齊全，所以 parent 有註冊不代表它有。
var knownSchedulerJobs = []string{"pre_market", "intraday", "daily_close", "sr_zone_verify", "chip_daily_sync", "stock_symbol_sync", "sr_evaluation", "corporate_action_sync", "evaluation_universe_sync", "candle_gap_detection", "sr_analysis", "sr_analysis_chip"}

// jobStaleThreshold 是各 job 預期的最大執行間隔，超過視為 stale（排程可能卡住或程式沒在跑）
var jobStaleThreshold = map[string]time.Duration{
	"pre_market":  26 * time.Hour,
	"intraday":    10 * time.Minute,
	"daily_close": 26 * time.Hour,
	// 跟著 daily_close 跑，所以門檻相同。
	"sr_zone_verify":    26 * time.Hour,
	"chip_daily_sync":   72 * time.Hour,
	"stock_symbol_sync": 26 * time.Hour,
	"sr_evaluation":     72 * time.Hour,
	// 平日 06:30 跑一次；跨週末最長間隔是週五到週一，加上緩衝取 80 小時。
	"corporate_action_sync": 80 * time.Hour,
	// 平日 16:00 跑一次，同樣跨週末，取 80 小時。
	// 本 job 與 sr_evaluation 預設關閉，未註冊時 GetStatus 回 disabled 而不套用這個門檻。
	"evaluation_universe_sync": 80 * time.Hour,
	// 跟著 evaluation_universe_sync 那輪跑，所以門檻相同。
	// 預設關閉；未註冊時 GetStatus 回 disabled 而不套用門檻——那正是「關閉」與
	// 「該跑卻沒跑」要分得開的原因。
	"candle_gap_detection": 80 * time.Hour,
	// SR 分析排程平日各跑一次（17:00 / 22:00），跨週末同樣取 80 小時。
	// 預設關閉，未註冊時 GetStatus 回 disabled 而不套用門檻。
	"sr_analysis":      80 * time.Hour,
	"sr_analysis_chip": 80 * time.Hour,
}

type jobStatus struct {
	JobName       string     `json:"job_name"`
	Status        string     `json:"status"`
	SymbolsTotal  int        `json:"symbols_total,omitempty"`
	SymbolsFailed int        `json:"symbols_failed,omitempty"`
	Error         string     `json:"error,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Stale         bool       `json:"stale"`
}

// GetStatus 回傳每個排程 job 最新一筆執行紀錄。沒有紀錄的 job 分成兩種：
// 排程有註冊的標 never_run（該跑卻沒跑），沒註冊的標 disabled（刻意沒開）。
// 完整規格見 docs/api-reference.md 的 GET /scheduler/status。
func (h *SchedulerHandler) GetStatus(c *gin.Context) {
	// **每個 job 各取最新一筆，不是「取最近 N 筆再分組」**：後者的視窗放不下一天的
	// 紀錄——intraday 每 5 分鐘一筆、一天 55 筆，會把早上跑過的 job 整批擠出去，
	// 於是 06:30 的 corporate_action_sync 每天過了 13:30 就被報成 never_run
	// ＋ stale（規格見 docs/api-reference.md 的 GET /scheduler/status）。
	// 查詢回的是「表裡有紀錄的 job_name 各一列」（沒跑過的不會出現），
	// 下面遍歷 knownSchedulerJobs 時才補成 never_run / disabled——
	// **固定回 len(knownSchedulerJobs) 列是這個迴圈的保證，不是那句 SQL 的**。
	runs, err := h.repo.GetLatestPerJob(c.Request.Context())
	if err != nil {
		serverError(c, h.log, err, "scheduler: get status")
		return
	}

	latest := make(map[string]store.JobRun, len(runs))
	for _, r := range runs {
		latest[r.JobName] = r
	}

	result := make([]jobStatus, 0, len(knownSchedulerJobs))
	for _, name := range knownSchedulerJobs {
		// 排程沒註冊就不是「卡住」，是「沒開」。兩者都沒有 job_runs 紀錄，
		// 但把刻意關閉的 job 標成 stale 會訓練使用者忽略這個旗標——
		// 真的有 job 卡住時反而看不出來（見 docs/issue.md 的收斂紀錄）。
		// 註冊與否由 scheduler 自己回報，不在這裡重算 config 條件。
		// 不對 h.sched 做 nil 檢查：本檔其餘 handler 全都直接 deref（`h.sched.RunDailyClose()`），
		// 只在這裡防禦並不一致，而 server.go 永遠傳非 nil。
		registered := h.sched.IsJobRegistered(name)

		r, ok := latest[name]
		if !ok {
			status := "never_run"
			if !registered {
				status = "disabled"
			}
			result = append(result, jobStatus{
				JobName: name,
				Status:  status,
				// 未註冊的 job 永遠不會有紀錄，標 stale 沒有意義
				Stale: registered,
			})
			continue
		}

		// 有歷史紀錄但排程已被關閉時同樣不算 stale——那是「跑過、後來關了」，
		// 不是「該跑卻沒跑」。
		stale := registered && r.Status != "running" && time.Since(r.StartedAt) > jobStaleThreshold[name]
		started := r.StartedAt
		js := jobStatus{
			JobName:       r.JobName,
			Status:        r.Status,
			SymbolsTotal:  r.SymbolsTotal,
			SymbolsFailed: r.SymbolsFailed,
			Error:         r.Error.String,
			StartedAt:     &started,
			Stale:         stale,
		}
		if r.FinishedAt.Valid {
			finished := r.FinishedAt.Time
			js.FinishedAt = &finished
		}
		result = append(result, js)
	}

	c.JSON(http.StatusOK, gin.H{"jobs": result})
}
