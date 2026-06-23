package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/trading/backend/internal/store"
)

type CandleHandler struct {
	repo store.CandleRepo
}

func NewCandleHandler(repo store.CandleRepo) *CandleHandler {
	return &CandleHandler{repo: repo}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":    symbol,
		"timeframe": timeframe,
		"candles":   candles,
	})
}
