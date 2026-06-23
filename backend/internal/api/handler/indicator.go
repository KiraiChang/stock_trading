package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/trading/backend/internal/store"
)

type IndicatorHandler struct {
	repo store.IndicatorRepo
}

func NewIndicatorHandler(repo store.IndicatorRepo) *IndicatorHandler {
	return &IndicatorHandler{repo: repo}
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
