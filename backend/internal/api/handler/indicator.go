package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/store"
)

type IndicatorHandler struct {
	engine *indicator.Engine
	repo   store.IndicatorRepo
}

func NewIndicatorHandler(engine *indicator.Engine, repo store.IndicatorRepo) *IndicatorHandler {
	return &IndicatorHandler{engine: engine, repo: repo}
}

func (h *IndicatorHandler) GetIndicators(c *gin.Context) {
	symbol := c.Param("symbol")
	timeframe := c.DefaultQuery("timeframe", "1d")

	snap, err := h.repo.GetLatest(c.Request.Context(), symbol, timeframe)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "indicators not found"})
		return
	}

	c.JSON(http.StatusOK, snap)
}

// Compute 手動計算單一股票的指標（POST /api/v1/indicators/:symbol/compute），
// 不要求該股票在監控清單裡，只要求 candles 至少 35 根（lookback 60 用的資料量）。
// 同步執行、直接回傳算出來的結果，不像 /market/backfill 是背景執行——這裡只算
// 一支股票，通常很快。
func (h *IndicatorHandler) Compute(c *gin.Context) {
	symbol := c.Param("symbol")
	timeframe := c.DefaultQuery("timeframe", "1d")

	snap, err := h.engine.Compute(c.Request.Context(), symbol, timeframe)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, snap)
}
