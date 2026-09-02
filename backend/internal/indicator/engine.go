package indicator

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

const lookback = 120 // 拉取最近 120 根 K 棒，足夠所有指標計算

// minCandles 是算得出指標的最低根數（MA60 需要 60 根，其餘指標更少）。
const minCandles = 35

type Engine struct {
	candles store.CandleRepo
	indRepo store.IndicatorRepo
	redis   *store.RedisClient
	log     *zap.Logger
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
	// **讀取失敗與資料不足是兩件事，不能合流**（原本兩者共用一個分支，而且
	// len(candles) < 35 但 err == nil 時 %w 包的是 nil）。呼叫端要靠 errors.Is
	// 分流成 5xx 與 422，見 errors.go 的說明。
	candles, err := e.candles.GetLatestN(ctx, symbol, timeframe, lookback)
	if err != nil {
		return nil, fmt.Errorf("load candles for %s/%s: %w", symbol, timeframe, err)
	}
	if len(candles) < minCandles {
		return nil, insufficientCandlesError(symbol, timeframe, len(candles))
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

	// **落盤是成功的必要條件（fail-fast）**：以前這裡只記 warn 就往下走，於是
	// Upsert 失敗後照樣寫 Redis、照樣把 snapshot 交給 signal engine——API 讀 DB 回舊值、
	// Redis 與 WebSocket 是新值，同一份資料依讀取路徑而不同。2026-09-01 的 2454
	// （rsi14 撞到 DECIMAL(6,4) 上限）整個盤中都是這個狀態，而 66 輪 intraday 全部
	// 回報 success。詳見 docs/architecture.md「寫入失敗的一致性契約」（原記於 issue.md I-102，已收斂）。
	if err := e.indRepo.Upsert(ctx, snap); err != nil {
		return nil, persistenceError(symbol, timeframe, err)
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
		"macd":  snap.MACD, "macd_signal": snap.MACDSignal, "macd_hist": snap.MACDHist,
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
