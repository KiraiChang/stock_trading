package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/joberr"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

type MarketHandler struct {
	fetcher      *market.Fetcher
	backfillJobs store.MarketBackfillJobRepo
	log          *zap.Logger
}

// NewMarketHandler 刻意不吃 WatchlistRepo：回補要補哪些股票由呼叫端明講，
// API 層不認識 watchlist（與 ChipHandler.Sync 的分工一致）。
// 「留空 ＝ 整個監控清單」是前端的語法糖，不是 API 行為。
func NewMarketHandler(fetcher *market.Fetcher, backfillJobs store.MarketBackfillJobRepo, log *zap.Logger) *MarketHandler {
	return &MarketHandler{fetcher: fetcher, backfillJobs: backfillJobs, log: log}
}

// Backfill 手動回補歷史日K（POST /api/v1/market/backfill）。
// Body: { "days": 120, "symbols": ["2330","2454"] }
//
// symbols 為**必填**——空陣列或缺鍵一律 400。先前會在此自動代入 watchlist，
// 造成「watchlist 為空」這種與回補無關的錯誤訊息，且讓這支端點無法用於
// watchlist 以外的標的（見 todo.md T-040）。
//
// 立即建立 market_backfill_jobs 紀錄後背景執行，回傳 job 供輪詢，比照
// ChipHandler.Sync：回補在 rate limit 下是長任務（20 檔約 4 分鐘），
// fire-and-forget 會讓前端完全看不到進度。
func (h *MarketHandler) Backfill(c *gin.Context) {
	var body struct {
		Days    int      `json:"days"`
		Symbols []string `json:"symbols"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Days <= 0 {
		body.Days = 120
	}
	if len(body.Symbols) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbols is required"})
		return
	}

	symbolsJSON, _ := json.Marshal(body.Symbols)
	job := &store.MarketBackfillJob{
		JobID:        newJobID("bf"),
		Symbols:      string(symbolsJSON),
		Days:         body.Days,
		Status:       "pending",
		SymbolsTotal: len(body.Symbols),
	}
	if err := h.backfillJobs.Create(c.Request.Context(), job); err != nil {
		serverError(c, h.log, err, "market: create backfill job")
		return
	}

	go h.runBackfill(job.JobID, body.Symbols, body.Days)

	c.JSON(http.StatusAccepted, gin.H{"job": job})
}

func (h *MarketHandler) runBackfill(jobID string, symbols []string, days int) {
	ctx := context.Background()
	// 這是 handler 起的 goroutine，gin 的 recovery middleware 管不到它：
	// 這裡若 panic（例如上游 client 的邊界情況）會直接帶掉整個 backend process。
	// 攔下來記 log 並把任務標成 failed，讓前端的輪詢能收斂而不是永遠停在 running。
	defer func() {
		if r := recover(); r != nil {
			h.log.Error("market backfill: panic recovered",
				zap.String("job_id", jobID), zap.Any("panic", r), zap.Stack("stack"))
			if err := h.backfillJobs.Finish(ctx, jobID, "failed", "internal error"); err != nil {
				h.log.Warn("market backfill: finish after panic failed",
					zap.String("job_id", jobID), zap.Error(err))
			}
		}
	}()

	done, failed := 0, 0
	failures := make([]map[string]string, 0)

	h.fetcher.BackfillHistory(ctx, symbols, days, func(symbol string, err error) {
		done++
		if err != nil {
			failed++
			// **failures 會被持久化並原樣渲染**（前端 Backfill.svelte 的
			// `{f.symbol}: {f.error}`），所以原因一律過 joberr 分類器；
			// 原始 cause 只進 log。
			h.log.Warn("market backfill: symbol failed", zap.String("symbol", symbol), zap.Error(err))
			failures = append(failures, map[string]string{
				"symbol": symbol,
				"error":  string(joberr.Classify(err)),
			})
		}
		failuresJSON, _ := json.Marshal(failures)
		if uerr := h.backfillJobs.UpdateProgress(ctx, jobID, done, failed, store.RawJSON(failuresJSON)); uerr != nil {
			h.log.Warn("market backfill: update progress failed", zap.String("job_id", jobID), zap.Error(uerr))
		}
	})

	status, errMsg := "done", ""
	switch {
	case failed > 0 && failed >= len(symbols):
		status, errMsg = "failed", "all symbols failed"
	case failed > 0:
		status, errMsg = "partial", "some symbols failed"
	}
	if err := h.backfillJobs.Finish(ctx, jobID, status, errMsg); err != nil {
		h.log.Warn("market backfill: finish job failed", zap.String("job_id", jobID), zap.Error(err))
	}
}

// GetBackfillJob 查詢回補任務進度（GET /api/v1/market/backfill/:job_id）。
func (h *MarketHandler) GetBackfillJob(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := h.backfillJobs.GetByJobID(c.Request.Context(), jobID)
	if err != nil {
		jobLookupError(c, h.log, err, "market: get backfill job")
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}
