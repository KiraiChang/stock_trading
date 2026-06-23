package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/trading/backend/internal/store"
)

type SignalHandler struct {
	repo store.SignalRepo
}

func NewSignalHandler(repo store.SignalRepo) *SignalHandler {
	return &SignalHandler{repo: repo}
}

func (h *SignalHandler) GetSignals(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	symbol := c.Query("symbol")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	var signals []store.Signal
	if symbol != "" {
		signals, err = h.repo.GetBySymbol(c.Request.Context(), symbol, limit)
	} else {
		signals, err = h.repo.GetRecent(c.Request.Context(), limit)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"signals": signals, "total": len(signals)})
}
