package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

type SRZoneHandler struct {
	client    *analysis.Client
	repo      store.SRZoneRepo
	watchlist store.WatchlistRepo
	log       *zap.Logger
}

func NewSRZoneHandler(client *analysis.Client, repo store.SRZoneRepo, watchlist store.WatchlistRepo, log *zap.Logger) *SRZoneHandler {
	return &SRZoneHandler{client: client, repo: repo, watchlist: watchlist, log: log}
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
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	a, zones, err := result.ToStore()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, err := h.repo.Create(c.Request.Context(), a, zones)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	saved, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	savedZones, err := h.repo.GetZones(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis": a, "zones": zones})
}

// POST /api/v1/sr-zones/train
// Body: { "symbols": ["2330","2454"], "timeframe": "1d", "limit": 1500, "model_type": "gradient_boosting" }
// symbols 省略時自動使用 watchlist 全部股票；在背景執行、立即回應
// （訓練可能耗時數十秒到數分鐘，見 analysis.Client.TrainModel 的說明）。
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if len(symbols) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "watchlist 為空；請先新增股票或在 request body 中指定 symbols"})
		return
	}

	go func() {
		result, err := h.client.TrainModel(context.Background(), symbols, body.Timeframe, body.Limit, body.ModelType)
		if err != nil {
			h.log.Error("sr_scoring train failed", zap.Int("symbols", len(symbols)), zap.Error(err))
			return
		}
		h.log.Info("sr_scoring train completed",
			zap.Int("rows", result.Rows), zap.Int("sources", result.Sources),
			zap.String("model_path", result.ModelPath), zap.Any("metrics", result.Metrics))
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "模型訓練已在背景啟動",
		"symbols": len(symbols),
	})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
