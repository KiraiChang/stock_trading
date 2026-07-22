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

var knownSchedulerJobs = []string{"pre_market", "intraday", "daily_close", "stock_symbol_sync"}

// jobStaleThreshold 是各 job 預期的最大執行間隔，超過視為 stale（排程可能卡住或程式沒在跑）
var jobStaleThreshold = map[string]time.Duration{
	"pre_market":        26 * time.Hour,
	"intraday":          10 * time.Minute,
	"daily_close":       26 * time.Hour,
	"stock_symbol_sync": 26 * time.Hour,
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

// GetStatus 回傳每個排程 job 最新一筆執行紀錄；未曾執行過的 job 標記為 never_run。
func (h *SchedulerHandler) GetStatus(c *gin.Context) {
	runs, err := h.repo.GetRecent(c.Request.Context(), 50)
	if err != nil {
		serverError(c, h.log, err, "scheduler: get status")
		return
	}

	latest := map[string]store.JobRun{}
	for _, r := range runs {
		if _, ok := latest[r.JobName]; !ok {
			latest[r.JobName] = r
		}
	}

	result := make([]jobStatus, 0, len(knownSchedulerJobs))
	for _, name := range knownSchedulerJobs {
		r, ok := latest[name]
		if !ok {
			result = append(result, jobStatus{JobName: name, Status: "never_run", Stale: true})
			continue
		}

		stale := r.Status != "running" && time.Since(r.StartedAt) > jobStaleThreshold[name]
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
