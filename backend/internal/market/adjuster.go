package market

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// Adjuster 維護 candles 的還原係數（見 docs/database-schema.md 的「股價還原」）。
//
// **設計的一句話**：`adj_factor` 是 `corporate_actions` 的純函數，重算永遠整段覆寫。
// 因此重複執行不會改變結果——這是使用者明確要求的性質，也是這個檔案所有寫法的理由。
//
// Phase 1 只處理分割。除權息（Phase 2）造成的是緩慢累積偏移，而且不調整成交量，
// 屆時會需要第二個係數。
type Adjuster struct {
	client     SplitSource
	dividends  DividendSource
	reductions CapitalReductionSource
	actions    store.CorporateActionRepo
	candles   store.CandleRepo
	log       *zap.Logger
}

// SetDividendSource 掛載除權息來源。未設定時該類事件不抓，行為與導入前相同。
func (a *Adjuster) SetDividendSource(d DividendSource) {
	a.dividends = d
}

// SetCapitalReductionSource 掛載減資來源。未設定時該類事件不抓。
func (a *Adjuster) SetCapitalReductionSource(r CapitalReductionSource) {
	a.reductions = r
}

// SplitSource 是分割事件的來源（目前是 FinMind 的 TaiwanStockSplitPrice）。
type SplitSource interface {
	FetchSplitPrices(ctx context.Context, start, end time.Time) ([]store.CorporateAction, error)
}

// DividendSource 是除權息事件的來源（目前是 Yahoo 的 dividendsByYear）。
// **逐檔查詢**——與分割不同，除權息沒有一次抓全市場的端點。
type DividendSource interface {
	FetchDividends(ctx context.Context, symbol string) ([]store.CorporateAction, error)
}

// CapitalReductionSource 是減資事件的來源（FinMind
// TaiwanStockCapitalReductionReferencePrice）。同樣**只能逐檔**。
type CapitalReductionSource interface {
	FetchCapitalReductions(ctx context.Context, symbol string) ([]store.CorporateAction, error)
}

func NewAdjuster(client SplitSource, actions store.CorporateActionRepo, candles store.CandleRepo, log *zap.Logger) *Adjuster {
	return &Adjuster{client: client, actions: actions, candles: candles, log: log}
}

// SyncSplits 抓取區間內的分割事件、寫入事件表，再重算受影響 symbol 的還原係數。
//
// 全市場 2015～2026 只有 33 筆分割，所以一次批次請求就抓得完整段歷史，
// 不需要逐檔也不需要增量——這正是 Phase 1 先做分割的原因。
func (a *Adjuster) SyncSplits(ctx context.Context, start, end time.Time) (int, error) {
	actions, err := a.client.FetchSplitPrices(ctx, start, end)
	if err != nil {
		return 0, fmt.Errorf("fetch splits: %w", err)
	}
	if err := a.actions.Upsert(ctx, actions); err != nil {
		return 0, fmt.Errorf("upsert corporate actions: %w", err)
	}
	a.log.Info("synced corporate actions", zap.Int("count", len(actions)))

	// 重算所有有事件的 symbol，而不是只重算這次抓到的——事件表可能被人工修過，
	// 而重算本來就是冪等的，多算幾檔只是多花一點時間，漏算才是問題。
	symbols, err := a.actions.Symbols(ctx)
	if err != nil {
		return 0, fmt.Errorf("list symbols: %w", err)
	}
	for _, symbol := range symbols {
		if err := a.RecomputeSymbol(ctx, symbol); err != nil {
			// 單一 symbol 失敗不該讓其他檔也停下來；重算是冪等的，下一輪會補上。
			a.log.Warn("recompute adj factor failed", zap.String("symbol", symbol), zap.Error(err))
		}
	}
	return len(actions), nil
}

// RecomputeAffected 只重算「有公司行動」的那些 symbol。
//
// 給回補用（見 docs/architecture/data-pipeline.md 的「公司行動同步」）：回補可能插入比事件更早的 K 棒，而 BulkInsert 寫入的
// adj_factor 是欄位預設值 1，那些列會靜靜地帶著未還原的價格直到隔天排程才修正。
//
// **刻意先過濾出有事件的檔**，而不是無腦對每個回補過的 symbol 呼叫 RecomputeSymbol：
// 後者對沒有事件的檔也會執行一次「整段歸零」的 UPDATE，回補 200 檔就是 200 次全表掃描，
// 而其中絕大多數本來就全是 1（全市場 33 筆事件只涵蓋 31 檔）。
func (a *Adjuster) RecomputeAffected(ctx context.Context, symbols []string) error {
	if len(symbols) == 0 {
		return nil
	}
	withEvents, err := a.actions.Symbols(ctx)
	if err != nil {
		return fmt.Errorf("list symbols: %w", err)
	}
	if len(withEvents) == 0 {
		return nil
	}
	target := make(map[string]bool, len(withEvents))
	for _, s := range withEvents {
		target[s] = true
	}

	var firstErr error
	for _, symbol := range symbols {
		if !target[symbol] {
			continue
		}
		if err := a.RecomputeSymbol(ctx, symbol); err != nil {
			a.log.Error("recompute after backfill failed",
				zap.String("symbol", symbol), zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// RecomputeSymbol 重算單一 symbol 的 adj_factor。
//
// 演算法（刻意寫成「先歸零、再由最新往回累乘」）：
//
//	adj_factor(bar) = Π factor(e)  for e in events where e.event_date > bar.ts
//
// 由後往前走事件，累積乘積就是「這根 K 棒之後還會發生的所有調整」。
// 最後一次事件之後的 K 棒沒有任何事件在它之後，係數為 1。
//
// **as-of 邊界**：`event_date` 是新價的第一個交易日（0050 的 2025-06-18 就已經是
// 分割後價格），所以套用範圍是 `ts < event_date`，事件當日**不套**。
func (a *Adjuster) RecomputeSymbol(ctx context.Context, symbol string) error {
	events, err := a.actions.ListBySymbol(ctx, symbol)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		// 沒有事件也要跑一次：事件被移除時要把殘留的係數清回 1。
		return a.candles.ApplyAdjFactors(ctx, symbol, nil)
	}

	// 由最新事件往回累乘。events 已依 event_date 升冪（repo 保證），順序固定，
	// 所以浮點連乘的結果每次都一樣。
	// **兩個累積係數**：價用全部事件，量只受改變股數的事件影響（見 T-042 Phase 2）。
	// 現金股利讓價格下修但股數沒變，volume_factor 為 1，連乘後不影響量。
	cumulativePrice, cumulativeVolume := 1.0, 1.0
	// upper 是本區間的右界（不含）：最後一個事件之後的 K 棒係數為 1，不必寫。
	upper := eventDay(events[len(events)-1])
	ranges := make([]store.AdjFactorRange, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		cumulativePrice *= events[i].Factor
		cumulativeVolume *= volumeFactorOrOne(events[i])
		var lower time.Time
		if i == 0 {
			// 最早的事件之前沒有更早的界，用零值時間涵蓋全部歷史。
			lower = time.Time{}
		} else {
			lower = eventDay(events[i-1])
		}
		ranges = append(ranges, store.AdjFactorRange{
			From: lower, To: upper, Price: cumulativePrice, Volume: cumulativeVolume,
		})
		upper = lower
	}
	cumulative := cumulativePrice

	// 歸零與覆寫在同一個交易內完成，讀取端不會在重算過程中看到未還原的價格。
	if err := a.candles.ApplyAdjFactors(ctx, symbol, ranges); err != nil {
		return err
	}

	// 一致性檢查：走到這裡，最早事件之前不該還有 adj_factor = 1 的 K 棒。
	// 這條抓的是「回補插入了比事件更早的 K 棒但沒重算」——那些列會靜靜地帶著未還原的價格。
	// 事件係數本身恰好等於 1 時不算異常（分割不可能是 1:1，Phase 2 的除權息才需要留意）。
	earliest := eventDay(events[0])
	if n, err := a.candles.CountUnadjustedBefore(ctx, symbol, earliest); err != nil {
		return err
	} else if n > 0 && cumulative != 1 {
		return fmt.Errorf("重算後 %s 仍有 %d 根 ts < %s 的 K 棒 adj_factor=1，係數未套用完整",
			symbol, n, earliest.Format("2006-01-02"))
	}
	return nil
}

// eventDay 把事件日正規化成台北時區的當日零點，與 candles.ts 的存法一致
// （抓取端用 ParseInLocation(..., TaipeiTZ)，所以一根日 K 的 ts 是台北當日 00:00）。
// 不正規化的話，事件日若帶了時分秒，邊界會偏移一根。
func eventDay(e store.CorporateAction) time.Time {
	t := e.EventDate.In(timeutil.TaipeiTZ)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, timeutil.TaipeiTZ)
}

// volumeFactorOrOne：Phase 1 寫入的事件沒有 volume_factor（欄位是 062 才加的），
// 讀到 0 時退回價格係數——Phase 1 只有分割，那時價量本來就共用一個係數。
func volumeFactorOrOne(e store.CorporateAction) float64 {
	if e.VolumeFactor > 0 {
		return e.VolumeFactor
	}
	return e.Factor
}

// SymbolsWithCandles 回傳所有有價格歷史的標的，供除權息同步決定要跑哪些檔。
func (a *Adjuster) SymbolsWithCandles(ctx context.Context) ([]string, error) {
	return a.candles.Symbols(ctx)
}

// SyncPerSymbolEvents 逐檔抓「沒有批次端點」的事件——除權息與減資——並重算。
//
// **兩者合併在同一個迴圈**是為了每檔只重算一次：重算要 UPDATE 該檔的整段歷史，
// 分開跑會做兩次。兩個來源打的是不同的 host（Yahoo／FinMind），各有各的節流器，
// 所以合併不會互相排擠。
//
// **刻意逐檔獨立處理**：任一來源對單一標的失敗（格式變動、被限流）不該讓整輪停下來。
// 失敗的檔維持前次係數（重算是冪等的），下一輪會自動補上。
func (a *Adjuster) SyncPerSymbolEvents(ctx context.Context, symbols []string) (int, error) {
	if len(symbols) == 0 || (a.dividends == nil && a.reductions == nil) {
		return 0, nil
	}
	total, failed := 0, 0
	for _, symbol := range symbols {
		var actions []store.CorporateAction
		symbolFailed := false

		if a.dividends != nil {
			got, err := a.dividends.FetchDividends(ctx, symbol)
			if err != nil {
				symbolFailed = true
				a.log.Warn("fetch dividends failed", zap.String("symbol", symbol), zap.Error(err))
			} else {
				actions = append(actions, got...)
			}
		}
		if a.reductions != nil {
			got, err := a.reductions.FetchCapitalReductions(ctx, symbol)
			if err != nil {
				symbolFailed = true
				a.log.Warn("fetch capital reductions failed", zap.String("symbol", symbol), zap.Error(err))
			} else {
				actions = append(actions, got...)
			}
		}
		if symbolFailed {
			failed++
		}
		if len(actions) == 0 {
			continue
		}
		if err := a.actions.Upsert(ctx, actions); err != nil {
			failed++
			a.log.Warn("upsert per-symbol events failed", zap.String("symbol", symbol), zap.Error(err))
			continue
		}
		total += len(actions)
		if err := a.RecomputeSymbol(ctx, symbol); err != nil {
			a.log.Error("recompute after per-symbol events failed",
				zap.String("symbol", symbol), zap.Error(err))
		}
	}
	if failed > 0 {
		a.log.Warn("逐檔事件同步有標的失敗",
			zap.Int("failed", failed), zap.Int("total", len(symbols)))
	}
	return total, nil
}
