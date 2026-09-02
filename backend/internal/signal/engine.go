package signal

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/chip"
	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/store"
)

// 【籌碼分析整合】訊號強度加權係數，見 applyChipWeighting 的規則說明。
const (
	chipStrengthBoost  = 1.3
	chipStrengthReduce = 0.6
	signalCooldown     = 15 * time.Minute
	recentSignalLimit  = 20
)

type Engine struct {
	candles store.CandleRepo
	signals store.SignalRepo
	redis   *store.RedisClient
	// emission 是本筆用到的三個 Redis 操作（reservation / enqueue / compare-delete）。
	// 拆成介面是為了讓測試注入可控 stub——`*store.RedisClient` 是具體型別，
	// 從 signal 的測試控制不了它的回應（見 docs/issue.md I-102「測試接縫」）。
	emission emissionStore
	// now 讓 TTL、每分鐘整掃與「Redis 恢復後仍被未到期 local reservation 抑制」
	// 這類時序測試不必靠 time.Sleep 賭。production 就是 time.Now。
	now               func() time.Time
	localReservations *localReservations
	indicator         *indicator.Engine
	chipScores        store.ChipScoreRepo
	log               *zap.Logger

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
		candles:           candles,
		signals:           signals,
		redis:             redis,
		emission:          redis,
		now:               time.Now,
		localReservations: newLocalReservations(),
		indicator:         ind,
		chipScores:        chipScores,
		log:               log,
	}
}

// SetEmissionStoreForTest / SetClockForTest 是測試接縫。
//
// ⛔ production 不得呼叫——constructor 已經接好真實的 Redis client 與 time.Now。
func (e *Engine) SetEmissionStoreForTest(s emissionStore) { e.emission = s }
func (e *Engine) SetClockForTest(now func() time.Time)    { e.now = now }

// Evaluate 對單一股票執行完整訊號分析，皆基於 candles（OHLCV）計算，不需要
// 額外的即時行情來源。回傳觸發的 Signal；沒有觸發（不符合突破/跌破/爆量
// 條件）時回傳 (nil, nil)，呼叫端可以用這個區分「沒訊號」跟「執行失敗」。
//
// **保留舊簽章供既有呼叫端使用**；要看降級資訊請用 EvaluateWithResult。
func (e *Engine) Evaluate(ctx context.Context, symbol, timeframe string) (*store.Signal, error) {
	res, err := e.EvaluateWithResult(ctx, symbol, timeframe)
	if err != nil {
		return nil, err
	}
	return res.Signal, nil
}

// EvaluateWithResult 與 Evaluate 相同，但回傳完整的 EvaluateResult。
//
// **degraded-success 的範圍是「局部失敗」，不是 DB outage**：Evaluate 第一行就是
// indicator.Compute，全域 DB 不可用時會在那裡就 return，根本走不到推播。
// 真正走得到這條路的是「indicator 落盤正常、但 signals 單表／單欄位寫入失敗」
// ——I-101 的 signals.vol_ratio 型別溢位就是那個形狀。詳見 docs/issue.md I-102。
func (e *Engine) EvaluateWithResult(ctx context.Context, symbol, timeframe string) (*EvaluateResult, error) {
	res := &EvaluateResult{}

	snap, err := e.indicator.Compute(ctx, symbol, timeframe)
	if err != nil {
		return res, err
	}

	candles, err := e.candles.GetLatestN(ctx, symbol, timeframe, 100)
	if err != nil || len(candles) == 0 {
		return res, err
	}
	if len(candles) < 2 {
		return res, nil
	}

	latestCandle := candles[len(candles)-1]
	supports, resistances := CalcSupportResistance(candles)
	trend := DetectTrend(candles)

	sig := CheckBreakout(symbol, snap, candles, resistances, supports, trend)
	if sig == nil {
		sig = CheckSupportBounce(symbol, snap, latestCandle, supports)
	}
	if sig == nil {
		return res, nil
	}

	e.applyChipWeighting(ctx, symbol, sig)

	// ── 判重：DB 為主、reservation 為輔 ───────────────────────────────
	// DB 判重讀的是權威歷史；但它失敗時會 fail-open，於是 signals 寫不進去時
	// 「判重」與「寫入」同時失效，每一輪都會重推同一個訊號。
	// reservation 是第二層，並且**通過 DB 判重時也要 reserve**——否則 cooldown
	// 中途 DB 才掛掉時，Redis 裡沒有那筆保留，重複推送照樣發生。
	suppress, err := e.shouldSuppressDuplicate(ctx, symbol, sig)
	if err != nil {
		e.log.Warn("signal duplicate check failed", zap.String("symbol", symbol), zap.Error(err))
		res.markDegraded(StageDedupDegraded, err)
	} else if suppress {
		e.log.Debug("duplicate signal suppressed",
			zap.String("symbol", symbol),
			zap.String("type", sig.SignalType),
			zap.String("direction", sig.Direction),
			zap.Time("timestamp", sig.Timestamp),
		)
		return res, nil
	}

	held, allowed := e.tryReserveEmission(ctx, signalIdentityKey(sig), res)
	if !allowed {
		e.log.Debug("duplicate signal suppressed by reservation",
			zap.String("symbol", symbol), zap.String("type", sig.SignalType))
		return res, nil
	}

	res.Signal = sig
	res.SignalGenerated = true

	// ── degraded-success：Insert 失敗仍然送出，但要看得見 ──────────────
	if err := e.signals.Insert(ctx, sig); err != nil {
		e.log.Warn("signal insert failed", zap.String("symbol", symbol), zap.Error(err))
		res.markDegraded(StageSignalPersistFailed, err)
	} else {
		res.DBPersisted = true
	}

	switch out := e.emission.EnqueueSignal(ctx, "signal:queue", sig); out.Status {
	case store.EnqueueEnqueued:
		res.QueueEnqueued = true
	case store.EnqueueDisabled:
		// 設定停用不是故障，不標降級。
	default:
		res.markDegraded(StageQueueFailed, out.Err)
	}

	if e.BroadcastFn != nil {
		e.BroadcastFn(symbol, sig)
		// **只代表嘗試過**——BroadcastFn 沒有回傳值，證明不了客戶端收到。
		res.BroadcastAttempted = true
	}

	// 兩個通道都沒送出去時釋放 reservation，讓下一輪能重試。
	//
	// ⚠️ **只有在 Insert 也失敗時才真的有效**：Insert 成功的話，下一輪會先被
	// DB 判重擋下（讀得到剛寫進去那列、又在 cooldown 內），根本不會重送。
	// 那個情形**明示接受不自動重送**——要保證投遞得另立 outbox／retry，
	// 那會把本筆從「錯誤處理語意」擴張成「投遞保證」。
	if !res.QueueEnqueued && !res.BroadcastAttempted {
		e.releaseEmission(ctx, held, res)
	}

	e.log.Info("signal generated",
		zap.String("symbol", symbol),
		zap.String("type", sig.SignalType),
		zap.String("direction", sig.Direction),
		zap.Float64("price", sig.Price),
		zap.Bool("db_persisted", res.DBPersisted),
		zap.Bool("degraded", res.Degraded),
	)
	return res, nil
}

func (e *Engine) shouldSuppressDuplicate(ctx context.Context, symbol string, sig *store.Signal) (bool, error) {
	recent, err := e.signals.GetBySymbol(ctx, symbol, recentSignalLimit)
	if err != nil {
		return false, err
	}

	for _, prev := range recent {
		if !sameSignalIdentity(prev, sig) {
			continue
		}
		elapsed := sig.Timestamp.Sub(prev.Timestamp)
		if elapsed >= 0 && elapsed < signalCooldown {
			return true, nil
		}
	}
	return false, nil
}

// sameSignalIdentity 比較兩個訊號是不是「同一個」。
//
// ⚠️ **它與 Redis reservation 共用同一個定義**（signalIdentityKey）：
// 原本這裡用容差比較、Redis key 用離散字串，兩套定義各自為政時，同一組價位可能
// DB 判「同一訊號」、Redis 判「不同訊號」，判重就出現破口。詳見 identity.go。
func sameSignalIdentity(prev store.Signal, next *store.Signal) bool {
	return signalIdentityKeyOf(prev) == signalIdentityKey(next)
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
