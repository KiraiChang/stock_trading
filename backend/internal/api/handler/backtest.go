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
func (h *BacktestHandler) Cancel(c *gin.Context) {
	jobID := c.Param("job_id")

	job, err := h.repo.GetJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	if job.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "only pending jobs can be cancelled"})
		return
	}
	if err := h.repo.UpdateJobStatus(c.Request.Context(), jobID, "failed", "cancelled by user"); err != nil {
		serverError(c, h.log, err, "backtest: cancel job")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "cancelled"})
}
