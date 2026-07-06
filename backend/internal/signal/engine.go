package signal

import (
	"context"
	"database/sql"
	"errors"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/chip"
	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/store"
)

// 【籌碼分析整合】訊號強度加權係數，見 applyChipWeighting 的規則說明。
const (
	chipStrengthBoost  = 1.3
	chipStrengthReduce = 0.6
)

type Engine struct {
	candles    store.CandleRepo
	signals    store.SignalRepo
	redis      *store.RedisClient
	indicator  *indicator.Engine
	chipScores store.ChipScoreRepo
	log        *zap.Logger

	// BroadcastFn 由 API 層注入，用於推送 WebSocket 事件
	BroadcastFn func(symbol string, sig *store.Signal)
}

func NewEngine(
	candles store.CandleRepo,
	signals store.SignalRepo,
	redis *store.RedisClient,
	ind *indicator.Engine,
	chipScores store.ChipScoreRepo,
	log *zap.Logger,
) *Engine {
	return &Engine{
		candles:    candles,
		signals:    signals,
		redis:      redis,
		indicator:  ind,
		chipScores: chipScores,
		log:        log,
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
		sig = CheckSupportBounce(symbol, snap, latestCandle, supports)
	}
	if sig == nil {
		return nil, nil
	}

	e.applyChipWeighting(ctx, symbol, sig)

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

// applyChipWeighting 依最新籌碼分數調整訊號強度（見
// docs/chip-analysis-design.md 第9節「與既有訊號和回測整合」）：
//   - BREAKOUT + 籌碼 BEARISH → 降低強度（技術面轉強但籌碼不認同）
//   - BREAKOUT + 籌碼 BULLISH → 提高強度（技術面+籌碼面同向）
//   - BREAKDOWN + 籌碼 BULLISH → 降低強度（技術轉弱但籌碼支持，訊號矛盾）
//   - BREAKDOWN + 籌碼 RISK → 只加註風險提示，不調整強度（RISK 本身就是
//     示警，不代表訊號方向被推翻）
//   - SUPPORT_BOUNCE + 籌碼 BULLISH → 提高強度（提高觀察優先度）
//
// 查無籌碼資料（sql.ErrNoRows）時直接略過，Strength 維持
// CheckBreakout/CheckSupportBounce 設定的預設值 1.0，不阻塞訊號產生——
// 籌碼資料是加分項，缺少不該讓整個 Evaluate 失敗。
func (e *Engine) applyChipWeighting(ctx context.Context, symbol string, sig *store.Signal) {
	score, err := e.chipScores.GetLatest(ctx, symbol)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			e.log.Warn("chip score lookup failed", zap.String("symbol", symbol), zap.Error(err))
		}
		return
	}

	sig.ChipSignal = store.NullString{NullString: sql.NullString{String: score.Signal, Valid: true}}

	switch sig.SignalType {
	case "BREAKOUT":
		switch score.Signal {
		case string(chip.Bearish):
			sig.Strength *= chipStrengthReduce
			sig.Note += "；籌碼偏空，訊號強度下修"
		case string(chip.Bullish):
			sig.Strength *= chipStrengthBoost
			sig.Note += "；籌碼偏多，訊號強度上修"
		}
	case "BREAKDOWN":
		switch score.Signal {
		case string(chip.Bullish):
			sig.Strength *= chipStrengthReduce
			sig.Note += "；籌碼偏多，訊號矛盾下修強度"
		case string(chip.Risk):
			sig.Note += "；融資使用率過高，風險升高"
		}
	case "SUPPORT_BOUNCE":
		if score.Signal == string(chip.Bullish) {
			sig.Strength *= chipStrengthBoost
			sig.Note += "；籌碼偏多，提高觀察優先度"
		}
	}
}
