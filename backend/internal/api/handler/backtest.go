package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/backtest"
	"github.com/trading/backend/internal/store"
)

type BacktestHandler struct {
	manager *backtest.Manager
	repo    store.BacktestRepo
	log     *zap.Logger
}

func NewBacktestHandler(manager *backtest.Manager, repo store.BacktestRepo, log *zap.Logger) *BacktestHandler {
	return &BacktestHandler{manager: manager, repo: repo, log: log}
}

// POST /api/v1/backtest
func (h *BacktestHandler) Submit(c *gin.Context) {
	var req backtest.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Strategy == "" || len(req.Symbols) == 0 || req.StartDate == "" || req.EndDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strategy, symbols, start_date, end_date are required"})
		return
	}
	// 日期、標的數、backtrader 單檔（I-137）：回的是跟 python 寫進 backtest_jobs.error 相同的代碼，前端對照成中文
	if code := backtest.ValidateRequest(req, h.manager.MaxSymbols()); code != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": code})
		return
	}
	req.Trigger = "manual"

	job, err := h.manager.Submit(c.Request.Context(), req)
	if err != nil {
		serverError(c, h.log, err, "backtest: submit")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"job": job})
}

// GET /api/v1/backtest
func (h *BacktestHandler) ListJobs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	jobs, err := h.repo.ListJobs(c.Request.Context(), limit)
	if err != nil {
		serverError(c, h.log, err, "backtest: list jobs")
		return
	}
	for i := range jobs {
		jobs[i].Error = backtest.PublicJobError(jobs[i].Error)
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs, "total": len(jobs)})
}

// GET /api/v1/backtest/:job_id
func (h *BacktestHandler) GetJob(c *gin.Context) {
	jobID := c.Param("job_id")

	job, err := h.repo.GetJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	// error 只回分類過的代碼（I-137）：I-137 之前留下的原始例外文字改回 internal_error
	job.Error = backtest.PublicJobError(job.Error)

	result, _ := h.repo.GetResult(c.Request.Context(), jobID) // 可能還沒完成

	c.JSON(http.StatusOK, gin.H{
		"job":    job,
		"result": result,
	})
}

// GET /api/v1/backtest/:job_id/trades
func (h *BacktestHandler) GetTrades(c *gin.Context) {
	jobID := c.Param("job_id")

	trades, err := h.repo.GetTrades(c.Request.Context(), jobID)
	if err != nil {
		serverError(c, h.log, err, "backtest: get trades")
		return
	}
	c.JSON(http.StatusOK, gin.H{"job_id": jobID, "trades": trades, "total": len(trades)})
}

// DELETE /api/v1/backtest/:job_id  (只能取消 pending 狀態)
//
// 條件式更新（只有 pending 才改），影響 1 列才算取消成功（200）。影響 0 列時再查一次：
// 不存在 → 404；存在（已被 python 認領或已結束）→ 409；任何 DB 錯誤 → 500（I-134）。
func (h *BacktestHandler) Cancel(c *gin.Context) {
	jobID := c.Param("job_id")
	ctx := c.Request.Context()

	cancelled, err := h.repo.CancelPendingJob(ctx, jobID)
	if err != nil {
		serverError(c, h.log, err, "backtest: cancel job")
		return
	}
	if cancelled {
		c.JSON(http.StatusOK, gin.H{"message": "cancelled"})
		return
	}
	if _, err := h.repo.GetJob(ctx, jobID); err != nil {
		jobLookupError(c, h.log, err, "backtest: cancel job lookup")
		return
	}
	c.JSON(http.StatusConflict, gin.H{"error": "only pending jobs can be cancelled"})
}
