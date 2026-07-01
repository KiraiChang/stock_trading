package signal

import (
	"context"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/store"
)

type Engine struct {
	candles   store.CandleRepo
	signals   store.SignalRepo
	redis     *store.RedisClient
	indicator *indicator.Engine
	log       *zap.Logger

	// BroadcastFn 由 API 層注入，用於推送 WebSocket 事件
	BroadcastFn func(symbol string, sig *store.Signal)
}

func NewEngine(
	candles store.CandleRepo,
	signals store.SignalRepo,
	redis *store.RedisClient,
	ind *indicator.Engine,
	log *zap.Logger,
) *Engine {
	return &Engine{
		candles:   candles,
		signals:   signals,
		redis:     redis,
		indicator: ind,
		log:       log,
	}
}

// Evaluate 對單一股票執行完整訊號分析，皆基於 candles（OHLCV）計算，不需要
// 額外的即時行情來源。回傳觸發的 Signal；沒有觸發（不符合突破/跌破/爆量
// 條件）時回傳 (nil, nil)，呼叫端可以用這個區分「沒訊號」跟「執行失敗」。
func (e *Engine) Evaluate(ctx context.Context, symbol, timeframe string) (*store.Signal, error) {
	snap, err := e.indicator.Compute(ctx, symbol, timeframe)
	if err != nil {
		return nil, err
	}

	candles, err := e.candles.GetLatestN(ctx, symbol, timeframe, 100)
	if err != nil || len(candles) == 0 {
		return nil, err
	}

	latestCandle := candles[len(candles)-1]
	supports, resistances := CalcSupportResistance(candles)
	trend := DetectTrend(candles)

	sig := CheckBreakout(symbol, snap, latestCandle, resistances, supports, trend)
	if sig == nil {
		return nil, nil
	}

	if err := e.signals.Insert(ctx, sig); err != nil {
		e.log.Warn("signal insert failed", zap.String("symbol", symbol), zap.Error(err))
	}

	e.redis.LPush(ctx, "signal:queue", sig)

	if e.BroadcastFn != nil {
		e.BroadcastFn(symbol, sig)
	}

	e.log.Info("signal generated",
		zap.String("symbol", symbol),
		zap.String("type", sig.SignalType),
		zap.String("direction", sig.Direction),
		zap.Float64("price", sig.Price),
	)
	return sig, nil
}

// EvaluateAll 批量掃描所有股票
func (e *Engine) EvaluateAll(ctx context.Context, symbols []string, timeframe string) {
	for _, sym := range symbols {
		if _, err := e.Evaluate(ctx, sym, timeframe); err != nil {
			e.log.Warn("evaluate failed", zap.String("symbol", sym), zap.Error(err))
		}
	}
}
