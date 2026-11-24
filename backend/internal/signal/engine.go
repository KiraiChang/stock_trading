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

// DedupPolicy 是盤中 1 分 K 的 SUPPORT_BOUNCE 用哪一條去重規則（I-140，docs/signal-spec.md「訊號去重」）。
// 其他類型與日 K 一律用 A；production 由 cmd/server 設定，⛔ 不開環境變數。
type DedupPolicy int

const (
	// DedupPriceKey（A，現行）：同一 symbol＋類型＋方向＋價位 15 分鐘內不重發。
	DedupPriceKey DedupPolicy = iota
	// DedupIntradayBounceBySymbol（B）：盤中 1 分 K 的 SUPPORT_BOUNCE，同一檔上一筆送出起算 15 分鐘內不再送出，⛔ 不比對支撐價位。
	DedupIntradayBounceBySymbol
)

type Engine struct {
	candles store.CandleRepo
	signals store.SignalRepo
	redis   *store.RedisClient
	// emission 是本筆用到的三個 Redis 操作（reservation / enqueue / compare-delete）。
	// 拆成介面是為了讓測試注入可控 stub——`*store.RedisClient` 是具體型別，
	// 從 signal 的測試控制不了它的回應。
	emission emissionStore
	// now 讓 TTL、每分鐘整掃與「Redis 恢復後仍被未到期 local reservation 抑制」
	// 這類時序測試不必靠 time.Sleep 賭。production 就是 time.Now。
	now               func() time.Time
	localReservations *localReservations
	dedupPolicy       DedupPolicy
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

// SetDedupPolicy 設定盤中 1 分 K 的 SUPPORT_BOUNCE 用哪一條去重規則（預設 A）。只在啟動時、開始評估之前呼叫。
func (e *Engine) SetDedupPolicy(p DedupPolicy) { e.dedupPolicy = p }

// usesIntradayBounceRule：這個候選是否走 B 規則（只有盤中 1 分 K 的 SUPPORT_BOUNCE，且設定為 B）。
func (e *Engine) usesIntradayBounceRule(sig *store.Signal, timeframe string) bool {
	return e.dedupPolicy == DedupIntradayBounceBySymbol && sig.SignalType == "SUPPORT_BOUNCE" && timeframe == intradayBounceTimeframe
}

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
// ——I-101 的 signals.vol_ratio 型別溢位就是那個形狀。詳見 docs/architecture.md「寫入失敗的一致性契約」。
func (e *Engine) EvaluateWithResult(ctx context.Context, symbol, timeframe string) (*EvaluateResult, error) {
	return e.EvaluateWithOptions(ctx, symbol, timeframe, EvaluateOptions{})
}

// EvaluateOptions 是 EvaluateWithOptions 的選項（T-088）。零值＝原本的行為。
type EvaluateOptions struct {
	// Withhold：照常算到判重為止，觸發時⛔ 不送出（⛔ 不 reserve、⛔ 不寫 signals、⛔ 不推播），
	// 結果記在 Withheld／WithheldSignal。盤中的資料比上一次評估新、但落後超過門檻時用：
	// 那時的訊號⛔ 不能當成即時訊號送出（docs/yahoo-intraday-integration.md）。
	Withhold bool
}

// EvaluateWithOptions 與 EvaluateWithResult 相同，多了 EvaluateOptions。
func (e *Engine) EvaluateWithOptions(ctx context.Context, symbol, timeframe string, opts EvaluateOptions) (*EvaluateResult, error) {
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
	// I-140：記下評估用的時間框架（所有類型）；B 規則的 DB 判重只看 timeframe = '1m' 的列。
	sig.Timeframe = store.NullString{NullString: sql.NullString{String: timeframe, Valid: true}}
	res.Candidate = sig
	bRule := e.usesIntradayBounceRule(sig, timeframe)
	// B 規則在 DB 判重**之前**取樣一次 now，DB、本機、Redis 與送出時間都用它（I-140；⛔ 不重新取樣：
	// DB 查詢在到期前開始、到期後才回來時，三條路徑仍用同一個 now，⛔ 不會跨秒分歧）。
	var now time.Time
	if bRule {
		now = e.now()
	}

	// ── 判重：DB 為主、reservation 為輔 ───────────────────────────────
	// DB 判重讀的是權威歷史；但它失敗時會 fail-open，於是 signals 寫不進去時
	// 「判重」與「寫入」同時失效，每一輪都會重推同一個訊號。
	// reservation 是第二層，並且**通過 DB 判重時也要 reserve**——否則 cooldown
	// 中途 DB 才掛掉時，Redis 裡沒有那筆保留，重複推送照樣發生。
	var suppress bool
	if bRule {
		suppress, err = e.shouldSuppressIntradayBounce(ctx, symbol, now)
	} else {
		suppress, err = e.shouldSuppressDuplicate(ctx, symbol, sig)
	}
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
		res.Suppressed = true
		res.SuppressedBy = SuppressedByDB
		return res, nil
	}

	// 不送出：停在 reservation 之前，⛔ 不佔用 reservation（否則資料追上之後那一輪會被自己擋下）。
	if opts.Withhold {
		res.Withheld = true
		res.WithheldSignal = sig
		return res, nil
	}

	var held *reservation
	var allowed bool
	if bRule {
		// 送出時間截到整秒（三種 DB 存得一樣），到期時刻＝送出時間＋15 分鐘，三條路徑共用。
		sentAt := now.Truncate(time.Second)
		held, allowed = e.tryReserveEmissionUntil(ctx, intradayBounceKey(sig), now, sentAt.Add(signalCooldown), res)
		sig.CreatedAt = sentAt
	} else {
		held, allowed = e.tryReserveEmission(ctx, signalIdentityKey(sig), res)
		sig.CreatedAt = e.now() // I-140：created_at 由程式寫入送出時間（repo 截到整秒）
	}
	if !allowed {
		e.log.Debug("duplicate signal suppressed by reservation",
			zap.String("symbol", symbol), zap.String("type", sig.SignalType))
		res.Suppressed = true
		res.SuppressedBy = SuppressedByReservation
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

// shouldSuppressIntradayBounce 是 B 規則的 DB 判重（I-140）：同一檔 timeframe = '1m' 最新的 SUPPORT_BOUNCE
// 是錨點，`now < 錨點的 created_at + 15 分鐘` 就抑制（now 比 created_at 早——時鐘倒退——也抑制）。
// 錨點只認「DB 已寫入」的列：被抑制、扣住（Withhold）的候選⛔ 不寫 signals，所以⛔ 不會延長冷卻。
func (e *Engine) shouldSuppressIntradayBounce(ctx context.Context, symbol string, now time.Time) (bool, error) {
	prev, err := e.signals.LatestBySymbolTypeTimeframe(ctx, symbol, "SUPPORT_BOUNCE", intradayBounceTimeframe)
	if err != nil || prev == nil {
		return false, err
	}
	return now.Before(prev.CreatedAt.Add(signalCooldown)), nil
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
