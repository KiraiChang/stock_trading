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

	res, err := h.engine.EvaluateWithResult(c.Request.Context(), symbol, timeframe)
	if err != nil {
		// **與 indicator 端點共用同一套分流**：Evaluate 第一行就是 indicator.Compute，
		// 同一個錯誤會從兩條 API 傳出去，狀態碼必須一致。
		indicatorComputeError(c, h.log, err, "signal: evaluate")
		return
	}
	// ⚠️ **HTTP 200 不再等於「已寫入 signal history」**——訊號可能已送出但沒落盤。
	// 呼叫端要看 db_persisted，不要用狀態碼推導。broadcast_attempted 的語意是
	// *delivery attempted*，不宣稱客戶端已收到（BroadcastFn 沒有回傳值）。
	//
	// ⚠️ **沒有訊號時也要回完整狀態**：DB 判重查詢失敗會標 dedup_degraded，
	// 之後若被 reservation 抑制，回應會是「沒有觸發訊號」——只回那句話的話，
	// 呼叫端完全看不到判重已經降級成單層。
	// degraded_stages **一律回陣列，沒有降級時是空陣列**。
	//
	// nil slice 在 map 裡會序列化成 `null`（struct 的 omitempty 對 map entry 無效），
	// 那會逼前端在迭代前先做 null 檢查——與 architecture.md「Nullable 欄位的 JSON
	// 序列化」要避免的是同一類問題：不要回一個需要額外防禦的形狀。
	stages := res.DegradedStages
	if stages == nil {
		stages = []signal.Stage{}
	}
	body := gin.H{
		"signal":              res.Signal,
		"signal_generated":    res.SignalGenerated,
		"db_persisted":        res.DBPersisted,
		"queue_enqueued":      res.QueueEnqueued,
		"broadcast_attempted": res.BroadcastAttempted,
		"degraded":            res.Degraded,
		"degraded_stages":     stages,
	}
	if !res.SignalGenerated {
		body["message"] = "沒有觸發訊號（不符合突破/跌破/爆量條件，或被判重抑制）"
	}
	c.JSON(http.StatusOK, body)
}
