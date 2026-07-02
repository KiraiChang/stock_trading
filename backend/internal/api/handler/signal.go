package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
)

type SignalHandler struct {
	engine *signal.Engine
	repo   store.SignalRepo
	log    *zap.Logger
}

func NewSignalHandler(engine *signal.Engine, repo store.SignalRepo, log *zap.Logger) *SignalHandler {
	return &SignalHandler{engine: engine, repo: repo, log: log}
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
		serverError(c, h.log, err, "signal: get signals")
		return
	}

	c.JSON(http.StatusOK, gin.H{"signals": signals, "total": len(signals)})
}

// Evaluate 手動觸發訊號評估（POST /api/v1/signals/:symbol/evaluate），完全
// 基於 candles（OHLCV）計算，不要求該股票在監控清單裡，也不需要即時行情。
// 常見用途：收盤後想立刻確認某支股票當天有沒有觸發訊號，不用等排程
// （daily_close 排程是 14:00 才對監控清單跑）。
func (h *SignalHandler) Evaluate(c *gin.Context) {
	symbol := c.Param("symbol")
	timeframe := c.DefaultQuery("timeframe", "1d")

	sig, err := h.engine.Evaluate(c.Request.Context(), symbol, timeframe)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if sig == nil {
		c.JSON(http.StatusOK, gin.H{"signal": nil, "message": "沒有觸發訊號（不符合突破/跌破/爆量條件）"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"signal": sig})
}
