package indicator

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

const lookback = 120 // 拉取最近 120 根 K 棒，足夠所有指標計算

type Engine struct {
	candles   store.CandleRepo
	indRepo   store.IndicatorRepo
	redis     *store.RedisClient
	log       *zap.Logger
}

func NewEngine(candles store.CandleRepo, indRepo store.IndicatorRepo, redis *store.RedisClient, log *zap.Logger) *Engine {
	return &Engine{
		candles: candles,
		indRepo: indRepo,
		redis:   redis,
		log:     log,
	}
}

// Compute 計算單一股票的所有指標，寫入 DB 與 Redis
func (e *Engine) Compute(ctx context.Context, symbol, timeframe string) (*store.IndicatorSnapshot, error) {
	candles, err := e.candles.GetLatestN(ctx, symbol, timeframe, lookback)
	if err != nil || len(candles) < 35 {
		return nil, fmt.Errorf("not enough candles for %s/%s: %w", symbol, timeframe, err)
	}

	closes := extractCloses(candles)
	highs := extractHighs(candles)
	lows := extractLows(candles)
	volumes := extractVolumes(candles)

	macdResult := CalcMACD(closes, 12, 26, 9)
	bbResult := CalcBollinger(closes, 20, 2.0)
	volResult := CalcVolumeSpike(volumes, 20)

	snap := &store.IndicatorSnapshot{
		Symbol:     symbol,
		Timeframe:  timeframe,
		Timestamp:  candles[len(candles)-1].Timestamp,
		MA5:        CalcMA(closes, 5),
		MA10:       CalcMA(closes, 10),
		MA20:       CalcMA(closes, 20),
		MA60:       CalcMA(closes, 60),
		RSI14:      CalcRSI(closes, 14),
		MACD:       macdResult.MACD,
		MACDSignal: macdResult.Signal,
		MACDHist:   macdResult.Histogram,
		BBUpper:    bbResult.Upper,
		BBMiddle:   bbResult.Middle,
		BBLower:    bbResult.Lower,
		ATR14:      CalcATR(highs, lows, closes, 14),
		VWAP:       CalcVWAP(highs, lows, closes, volumes),
		VolMA20:    volResult.MA20,
		VolRatio:   volResult.Ratio,
	}

	if err := e.indRepo.Upsert(ctx, snap); err != nil {
		e.log.Warn("indicator upsert failed", zap.String("symbol", symbol), zap.Error(err))
	}

	e.cacheToRedis(ctx, symbol, timeframe, snap)

	return snap, nil
}

// ComputeAll 批量計算多個股票的指標
func (e *Engine) ComputeAll(ctx context.Context, symbols []string, timeframe string) error {
	for _, sym := range symbols {
		if _, err := e.Compute(ctx, sym, timeframe); err != nil {
			e.log.Warn("compute failed", zap.String("symbol", sym), zap.Error(err))
		}
	}
	return nil
}

func (e *Engine) cacheToRedis(ctx context.Context, symbol, timeframe string, snap *store.IndicatorSnapshot) {
	key := fmt.Sprintf("indicator:%s:%s:latest", symbol, timeframe)
	values := map[string]interface{}{
		"ma5": snap.MA5, "ma10": snap.MA10, "ma20": snap.MA20, "ma60": snap.MA60,
		"rsi14": snap.RSI14,
		"macd": snap.MACD, "macd_signal": snap.MACDSignal, "macd_hist": snap.MACDHist,
		"bb_upper": snap.BBUpper, "bb_middle": snap.BBMiddle, "bb_lower": snap.BBLower,
		"atr14": snap.ATR14, "vwap": snap.VWAP,
		"vol_ma20": snap.VolMA20, "vol_ratio": snap.VolRatio,
		"ts": snap.Timestamp.Unix(),
	}
	if err := e.redis.HSet(ctx, key, values); err != nil {
		e.log.Warn("redis hset failed", zap.String("key", key), zap.Error(err))
		return
	}
	e.redis.Expire(ctx, key, 5*time.Minute)
}

func extractCloses(cs []store.Candle) []float64 {
	out := make([]float64, len(cs))
	for i, c := range cs {
		out[i] = c.Close
	}
	return out
}

func extractHighs(cs []store.Candle) []float64 {
	out := make([]float64, len(cs))
	for i, c := range cs {
		out[i] = c.High
	}
	return out
}

func extractLows(cs []store.Candle) []float64 {
	out := make([]float64, len(cs))
	for i, c := range cs {
		out[i] = c.Low
	}
	return out
}

func extractVolumes(cs []store.Candle) []int64 {
	out := make([]int64, len(cs))
	for i, c := range cs {
		out[i] = c.Volume
	}
	return out
}
