package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

// mapScoreZonesError 依 Python /sr-zones 回傳的實際狀態碼，回給前端對應的
// 通用訊息——不透漏原始錯誤文字（細節只寫 log），但至少讓前端能分辨是
// 「該補資料」「該去訓練模型」還是「Python service 沒開」，不是每種情況
// 都顯示同一句「Python service 錯誤」。
func mapScoreZonesError(c *gin.Context, log *zap.Logger, err error) {
	var upstreamErr *analysis.UpstreamStatusError
	if errors.As(err, &upstreamErr) {
		switch upstreamErr.StatusCode {
		case http.StatusNotFound:
			log.Warn("sr-zones: score zones (no candles)", zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "找不到歷史資料，請確認股票代號是否正確，或先用「歷史資料回補」補資料"})
			return
		case http.StatusServiceUnavailable:
			log.Warn("sr-zones: score zones (model not trained)", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "機率模型尚未訓練，請先在下方「訓練/更新機率模型」區塊訓練"})
			return
		}
	}
	log.Error("sr-zones: score zones", zap.Error(err))
	c.JSON(http.StatusBadGateway, gin.H{"error": "Python 服務無法連線，請確認服務是否已啟動"})
}

type SRZoneHandler struct {
	client    *analysis.Client
	repo      store.SRZoneRepo
	watchlist store.WatchlistRepo
	trainJobs store.SRScoringTrainJobRepo
	verifier  *analysis.SRZoneVerifier
	log       *zap.Logger
}

func NewSRZoneHandler(
	client *analysis.Client, repo store.SRZoneRepo, watchlist store.WatchlistRepo,
	trainJobs store.SRScoringTrainJobRepo, verifier *analysis.SRZoneVerifier, log *zap.Logger,
) *SRZoneHandler {
	return &SRZoneHandler{client: client, repo: repo, watchlist: watchlist, trainJobs: trainJobs, verifier: verifier, log: log}
}

// newTrainJobID 比照 backtest.newJobID 的時間戳格式，不同前綴以便從 job_id
// 一眼分辨來源。
func newTrainJobID() string {
	return "sr_train_" + time.Now().UTC().Format("20060102_150405_000")
}

// POST /api/v1/sr-zones
// Body: { "symbol": "2330", "timeframe": "1d", "limit": 250 }
// limit 省略或為 0 時使用 Python 端的預設值（DEFAULT_FETCH_LIMIT）
func (h *SRZoneHandler) Create(c *gin.Context) {
	var body struct {
		Symbol    string `json:"symbol"`
		Timeframe string `json:"timeframe"`
		Limit     int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}
	if body.Timeframe == "" {
		body.Timeframe = "1d"
	}
	if body.Limit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be >= 0"})
		return
	}

	result, err := h.client.ScoreZones(c.Request.Context(), body.Symbol, body.Timeframe, body.Limit)
	if err != nil {
		mapScoreZonesError(c, h.log, err)
		return
	}

	a, zones, err := result.ToStore()
	if err != nil {
		serverError(c, h.log, err, "sr-zones: convert result to store")
		return
	}

	id, err := h.repo.Create(c.Request.Context(), a, zones)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: create analysis")
		return
	}

	saved, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: get saved analysis")
		return
	}
	savedZones, err := h.repo.GetZones(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: get saved zones")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"analysis": saved, "zones": savedZones})
}

// GET /api/v1/sr-zones?symbol=2330&limit=20
func (h *SRZoneHandler) List(c *gin.Context) {
	symbol := c.Query("symbol")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	rows, err := h.repo.List(c.Request.Context(), symbol, limit)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: list analyses")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analyses": rows, "total": len(rows)})
}

// GET /api/v1/sr-zones/:id
func (h *SRZoneHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	a, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sr zone analysis not found"})
		return
	}
	zones, err := h.repo.GetZones(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: get zones")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis": a, "zones": zones})
}

// POST /api/v1/sr-zones/:id/verify
// 手動重新驗證：比對這筆分析之後的實際 K 棒，更新每個 zone 的 status（是否
// 被突破）。可重複呼叫，每次都用目前為止最新的資料重新計算，不是一次性
// 判定（見 internal/analysis/sr_zone_verifier.go）。沒有自動排程，需要主動
// 呼叫這支 API 才會更新（但 daily_close 排程會自動對近期分析跑一次，見
// scheduler）。
func (h *SRZoneHandler) Verify(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	a, zones, err := h.verifier.Verify(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: verify")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis": a, "zones": zones})
}

// POST /api/v1/sr-zones/train
// Body: { "symbols": ["2330","2454"], "timeframe": "1d", "limit": 1500, "model_type": "gradient_boosting" }
// symbols 省略時自動使用 watchlist 全部股票；立即建立一筆 sr_scoring_train_jobs
// 紀錄（status=pending）並回傳 job_id，實際訓練在背景 goroutine 執行（可能
// 耗時數十秒到數分鐘，見 analysis.Client.TrainModel 的說明），呼叫端可用
// job_id 輪詢 GET /sr-zones/train-jobs/:job_id 查詢進度，不需要只靠伺服器
// log 才知道訓練有沒有成功。
func (h *SRZoneHandler) Train(c *gin.Context) {
	var body struct {
		Symbols   []string `json:"symbols"`
		Timeframe string   `json:"timeframe"`
		Limit     int      `json:"limit"`
		ModelType string   `json:"model_type"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Timeframe == "" {
		body.Timeframe = "1d"
	}
	if body.ModelType == "" {
		body.ModelType = "gradient_boosting"
	}

	symbols := body.Symbols
	if len(symbols) == 0 {
		var err error
		symbols, err = h.watchlist.Symbols(c.Request.Context())
		if err != nil {
			serverError(c, h.log, err, "sr-zones: list watchlist symbols for train")
			return
		}
	}
	if len(symbols) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "watchlist 為空；請先新增股票或在 request body 中指定 symbols"})
		return
	}

	symbolsJSON, err := json.Marshal(symbols)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: marshal train symbols")
		return
	}

	jobID := newTrainJobID()
	job := &store.SRScoringTrainJob{
		JobID:      jobID,
		Symbols:    string(symbolsJSON),
		Timeframe:  body.Timeframe,
		FetchLimit: body.Limit,
		ModelType:  body.ModelType,
	}
	if _, err := h.trainJobs.Create(c.Request.Context(), job); err != nil {
		serverError(c, h.log, err, "sr-zones: create train job")
		return
	}

	go h.runTrainJob(jobID, symbols, body.Timeframe, body.Limit, body.ModelType)

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  jobID,
		"status":  "pending",
		"message": "模型訓練已在背景啟動",
		"symbols": len(symbols),
	})
}

// runTrainJob 在背景 goroutine 執行，不使用 request context（request 結束後
// context 就會被取消，但訓練要繼續跑到完成）。狀態更新失敗只記 log——訓練
// 本身的成敗不應該因為「記錄狀態」這個次要動作失敗而受影響。
func (h *SRZoneHandler) runTrainJob(jobID string, symbols []string, timeframe string, limit int, modelType string) {
	ctx := context.Background()
	if err := h.trainJobs.MarkRunning(ctx, jobID); err != nil {
		h.log.Error("sr_scoring train job: mark running failed", zap.String("job_id", jobID), zap.Error(err))
	}

	result, err := h.client.TrainModel(ctx, symbols, timeframe, limit, modelType)
	if err != nil {
		h.log.Error("sr_scoring train failed", zap.String("job_id", jobID), zap.Int("symbols", len(symbols)), zap.Error(err))
		if markErr := h.trainJobs.MarkFailed(ctx, jobID, err.Error()); markErr != nil {
			h.log.Error("sr_scoring train job: mark failed failed", zap.String("job_id", jobID), zap.Error(markErr))
		}
		return
	}

	metricsJSON, err := json.Marshal(result.Metrics)
	if err != nil {
		h.log.Error("sr_scoring train job: marshal metrics failed", zap.String("job_id", jobID), zap.Error(err))
		metricsJSON = []byte("{}")
	}
	datasetSummaryJSON, err := json.Marshal(result.DatasetSummary)
	if err != nil {
		h.log.Error("sr_scoring train job: marshal dataset summary failed", zap.String("job_id", jobID), zap.Error(err))
		datasetSummaryJSON = []byte("{}")
	}
	if err := h.trainJobs.MarkDone(
		ctx, jobID, result.Rows, result.Sources, store.RawJSON(metricsJSON), result.ModelPath, result.Version,
		store.RawJSON(datasetSummaryJSON),
	); err != nil {
		h.log.Error("sr_scoring train job: mark done failed", zap.String("job_id", jobID), zap.Error(err))
	}
	h.log.Info("sr_scoring train completed",
		zap.String("job_id", jobID), zap.Int("rows", result.Rows), zap.Int("sources", result.Sources),
		zap.String("model_path", result.ModelPath), zap.Any("metrics", result.Metrics))
}

// GET /api/v1/sr-zones/train-jobs?limit=20
func (h *SRZoneHandler) ListTrainJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	jobs, err := h.trainJobs.List(c.Request.Context(), limit)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: list train jobs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs, "total": len(jobs)})
}

// GET /api/v1/sr-zones/train-jobs/:job_id
func (h *SRZoneHandler) GetTrainJob(c *gin.Context) {
	jobID := c.Param("job_id")

	job, err := h.trainJobs.Get(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "train job not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}

// GET /api/v1/sr-zones/model-status
// 讓前端在觸發分析前就能知道模型準備好了沒，不用先按分析失敗才知道
// （見 sr-zone-scoring.md「模型可追蹤性」）。永遠回 200，用 body 裡的
// exists 欄位表示模型存不存在。
func (h *SRZoneHandler) ModelStatus(c *gin.Context) {
	status, err := h.client.GetModelStatus(c.Request.Context())
	if err != nil {
		badGatewayError(c, h.log, err, "sr-zones: get model status")
		return
	}
	c.JSON(http.StatusOK, status)
}

// DELETE /api/v1/sr-zones/:id
func (h *SRZoneHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if _, err := h.repo.Get(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sr zone analysis not found"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		serverError(c, h.log, err, "sr-zones: delete analysis")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
