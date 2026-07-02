package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

type WatchlistHandler struct {
	repo store.WatchlistRepo
	log  *zap.Logger
}

func NewWatchlistHandler(repo store.WatchlistRepo, log *zap.Logger) *WatchlistHandler {
	return &WatchlistHandler{repo: repo, log: log}
}

func (h *WatchlistHandler) GetAll(c *gin.Context) {
	items, err := h.repo.GetAll(c.Request.Context())
	if err != nil {
		serverError(c, h.log, err, "watchlist: get all")
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
		serverError(c, h.log, err, "watchlist: add")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "added"})
}

// POST /api/v1/watchlist/bulk
// Body: { "items": [{"symbol":"2330","name":"台積電","sector":"半導體"}, ...] }
func (h *WatchlistHandler) BulkAdd(c *gin.Context) {
	var body struct {
		Items []struct {
			Symbol string `json:"symbol" binding:"required"`
			Name   string `json:"name"   binding:"required"`
			Sector string `json:"sector"`
		} `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	added, failed := 0, 0
	for _, item := range body.Items {
		if err := h.repo.Add(c.Request.Context(), item.Symbol, item.Name, item.Sector); err != nil {
			failed++
		} else {
			added++
		}
	}
	c.JSON(http.StatusCreated, gin.H{"added": added, "failed": failed, "total": len(body.Items)})
}

func (h *WatchlistHandler) Update(c *gin.Context) {
	symbol := c.Param("symbol")
	var body struct {
		Name   string `json:"name"   binding:"required"`
		Sector string `json:"sector"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.Update(c.Request.Context(), symbol, body.Name, body.Sector); err != nil {
		serverError(c, h.log, err, "watchlist: update")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *WatchlistHandler) Remove(c *gin.Context) {
	symbol := c.Param("symbol")
	if err := h.repo.Remove(c.Request.Context(), symbol); err != nil {
		serverError(c, h.log, err, "watchlist: remove")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}

// PATCH /api/v1/watchlist/:symbol/watch
// Body: { "watched": true }
// 最多同時監聽 store.MaxWatchedSymbols 檔，超過會回 409。
func (h *WatchlistHandler) SetWatched(c *gin.Context) {
	symbol := c.Param("symbol")
	var body struct {
		Watched bool `json:"watched"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.SetWatched(c.Request.Context(), symbol, body.Watched); err != nil {
		if errors.Is(err, store.ErrWatchLimitExceeded) {
			c.JSON(http.StatusConflict, gin.H{"error": "已達監聽上限（3 檔），請先取消其他股票的監聽"})
			return
		}
		serverError(c, h.log, err, "watchlist: set watched")
		return
	}
	c.JSON(http.StatusOK, gin.H{"symbol": symbol, "watched": body.Watched})
}
