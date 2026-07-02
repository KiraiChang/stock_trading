package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

type CandleHandler struct {
	repo store.CandleRepo
	log  *zap.Logger
}

func NewCandleHandler(repo store.CandleRepo, log *zap.Logger) *CandleHandler {
	return &CandleHandler{repo: repo, log: log}
}

func (h *CandleHandler) GetCandles(c *gin.Context) {
	symbol := c.Param("symbol")
	timeframe := c.DefaultQuery("timeframe", "1d")
	limitStr := c.DefaultQuery("limit", "60")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 1000 {
		limit = 60
	}

	candles, err := h.repo.GetLatestN(c.Request.Context(), symbol, timeframe, limit)
	if err != nil {
		serverError(c, h.log, err, "candle: get latest")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":    symbol,
		"timeframe": timeframe,
		"candles":   candles,
	})
}
