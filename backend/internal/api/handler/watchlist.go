package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/trading/backend/internal/store"
)

type WatchlistHandler struct {
	repo store.WatchlistRepo
}

func NewWatchlistHandler(repo store.WatchlistRepo) *WatchlistHandler {
	return &WatchlistHandler{repo: repo}
}

func (h *WatchlistHandler) GetAll(c *gin.Context) {
	items, err := h.repo.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"watchlist": items})
}

func (h *WatchlistHandler) Add(c *gin.Context) {
	var body struct {
		Symbol string `json:"symbol" binding:"required"`
		Name   string `json:"name"   binding:"required"`
		Sector string `json:"sector"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.Add(c.Request.Context(), body.Symbol, body.Name, body.Sector); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "added"})
}

func (h *WatchlistHandler) Remove(c *gin.Context) {
	symbol := c.Param("symbol")
	if err := h.repo.Remove(c.Request.Context(), symbol); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}
