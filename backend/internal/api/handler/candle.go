package handler

import (
	"net/http"
	"strconv"
	"time"

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

	// K 線圖用還原價（見 docs/todo.md T-042）。**調整在這裡做完，前端不做任何換算**：
	// 前端是第三個語言，讓它自己乘係數等於把同一段邏輯散到三處，而 TypeScript 那側
	// 算錯不會有任何東西告訴你。
	//
	// 不加 ?adjusted=false 之類的開關：目前沒有任何呼叫端需要從這支端點拿原始價，
	// 需要「當時實際成交在哪裡」的地方走的是報價／分析端點。等真的有消費者再加。
	adjusted := make([]adjustedCandle, 0, len(candles))
	for _, k := range candles {
		adjusted = append(adjusted, adjustedCandle{
			Symbol:    k.Symbol,
			Timeframe: k.Timeframe,
			Open:      k.AdjustedOpen(),
			High:      k.AdjustedHigh(),
			Low:       k.AdjustedLow(),
			Close:     k.AdjustedClose(),
			Volume:    k.AdjustedVolume(),
			Amount:    k.Amount,
			Timestamp: k.Timestamp,
			AdjFactor: k.AdjFactor,
			VolFactor: k.VolFactor,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":    symbol,
		"timeframe": timeframe,
		"candles":   adjusted,
	})
}

// adjustedCandle 是 K 線圖用的還原後 K 棒。
//
// 欄位名與 store.Candle 的 JSON 一致，前端不需要改欄位對應；差別只在 open/high/low/close
// 已經是還原價、volume 已經是還原量。`amount` 不調整（成交金額是錢，不隨股數重新定義），
// `adj_factor` 一併回傳，讓呼叫端看得出這根有沒有被調整過。
//
// volume 型別從 int64 變成 float64：還原量是除法的結果，取整會讓
// `adj_close * adj_volume == close * volume` 這條恆等式不成立。
type adjustedCandle struct {
	Symbol    string    `json:"symbol"`
	Timeframe string    `json:"timeframe"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"ts"`
	AdjFactor float64   `json:"adj_factor"`
	// 成交量用的是 vol_factor 而不是 adj_factor：現金股利改價不改股數。
	VolFactor float64 `json:"vol_factor"`
}
