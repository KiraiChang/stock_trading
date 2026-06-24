package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

type MarketHandler struct {
	fetcher   *market.Fetcher
	watchlist store.WatchlistRepo
	log       *zap.Logger
}

func NewMarketHandler(fetcher *market.Fetcher, watchlist store.WatchlistRepo, log *zap.Logger) *MarketHandler {
	return &MarketHandler{fetcher: fetcher, watchlist: watchlist, log: log}
}

// POST /api/v1/market/backfill
// Body: { "days": 120, "symbols": ["2330","2454"] }
// symbols 省略時自動使用 watchlist 全部股票；days 預設 120
func (h *MarketHandler) Backfill(c *gin.Context) {
	var body struct {
		Days    int      `json:"days"`
		Symbols []string `json:"symbols"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Days <= 0 {
		body.Days = 120
	}

	ctx := context.Background()
	symbols := body.Symbols
	if len(symbols) == 0 {
		var err error
		symbols, err = h.watchlist.Symbols(ctx)
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
		if err := h.fetcher.BackfillHistory(context.Background(), symbols, body.Days); err != nil {
			h.log.Warn("backfill failed", zap.Error(err))
			return
		}
		h.log.Info("backfill completed", zap.Int("symbols", len(symbols)), zap.Int("days", body.Days))
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "backfill 已在背景啟動",
		"symbols": len(symbols),
		"days":    body.Days,
	})
}
