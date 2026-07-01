package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/trading/backend/internal/store"
)

type SchedulerHandler struct {
	repo store.JobRunRepo
}

func NewSchedulerHandler(repo store.JobRunRepo) *SchedulerHandler {
	return &SchedulerHandler{repo: repo}
}

var knownSchedulerJobs = []string{"pre_market", "intraday", "daily_close"}

// jobStaleThreshold 是各 job 預期的最大執行間隔，超過視為 stale（排程可能卡住或程式沒在跑）
var jobStaleThreshold = map[string]time.Duration{
	"pre_market":  26 * time.Hour,
	"intraday":    10 * time.Minute,
	"daily_close": 26 * time.Hour,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			Error:         r.Error,
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
