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
//
// 回傳 `(processed, failed, err)`（2026-08-24 起的簽章）：
//
//   - `processed` 是**已經處理完的標的數**，不是事件筆數。呼叫端要拿它跟計畫要跑的
//     標的數相比，才知道有沒有跑完；事件筆數只有 log 價值，改由 log 輸出。
//   - `failed` 是**失敗的標的數**，同一檔不論在哪個階段失敗都只計一次——它會被寫進
//     `job_runs.symbols_failed`（單位是標的數），重複計數會讓 `failed >= total` 的
//     狀態判定失準（欄位語意見 docs/api-reference.md 的 /scheduler/status）。**抓取、寫入、重算三個階段都算失敗**：
//     事件寫進去了但係數沒重算完，那檔的價格是不一致的，比抓不到事件更嚴重
//     （2026-08-24 review 補；先前重算失敗只寫 log）。
//   - `err` 只在 ctx 被取消／逾時時非 nil：迴圈頂端先檢查 `ctx.Err()`，一旦非 nil
//     立刻 `break`。修掉的是「逾時後仍把剩下 800 多檔跑完、每檔發兩次注定失敗的請求
//     並各記一行 warn」。此時 `processed` 停在實際跑完的檔數，
//     未處理的部分由呼叫端換算。deadline 落在**最後一檔**的請求裡時不會再進下一輪，
//     所以收尾另補一次檢查，否則會出現「partial 但 error 欄空白」
//     （2026-08-24 review 補）。收尾檢查只在**有某一檔的失敗發生在 ctx 到期之後**時才歸因給
//     ctx——單看整輪 `failed > 0` 會把「先前的一般失敗 ＋ 收尾後才到期」誤標成逾時
//     （2026-08-24 review 補）；而那個判斷要**在各失敗分支當場採樣**，等整檔跑完才採樣一樣會誤判
//     （同檔「dividends 一般失敗 → reductions 成功 → ctx 才到期」）。個別標的的失敗原因
//     不往上傳，只進 log。
func (a *Adjuster) SyncPerSymbolEvents(ctx context.Context, symbols []string) (int, int, error) {
	if len(symbols) == 0 || (a.dividends == nil && a.reductions == nil) {
		return 0, 0, nil
	}
	events, processed, failed := 0, 0, 0
	var ctxErr error
	// deadlineHit 記「有沒有哪一檔的失敗發生在 ctx 已到期之後」，由下面的 markFailed
	// **在各失敗分支當場採樣**。
	//
	// 不用 errors.Is 檢查各階段的錯誤，是因為 DB 那兩個階段穿不過去：
	// lib/pq 被取消時回的是 `pq: canceling statement due to user request`，
	// errors.Is(err, context.Canceled) 為 false。改用「失敗時 ctx 是否已到期」不依賴
	// 錯誤包裝，四個階段一體適用。
	//
	// **限度**：操作因一般錯誤回傳、到 markFailed 讀 ctx.Err() 之間仍有幾奈秒的窗口，
	// 那段時間內到期的話還是會誤歸因。要完全消除得靠錯誤鏈判斷，而那條被 lib/pq 擋住，
	// 所以這裡是把窗口壓到最小，不是消滅它。
	deadlineHit := false
	for _, symbol := range symbols {
		// 逾時後不再送出注定失敗的請求：停在這裡，未處理數由呼叫端從 processed 換算。
		if err := ctx.Err(); err != nil {
			ctxErr = err
			break
		}

		var actions []store.CorporateAction
		symbolFailed := false
		// markFailed 把「這檔失敗了」與「失敗時 ctx 是不是已經到期」綁在一起採樣。
		// **取樣點必須在失敗分支裡**：等整檔四個階段跑完才採樣的話，
		// 「dividends 一般失敗 → reductions 成功 → ctx 才到期」會被誤歸因給預算
		// （2026-08-24 review 的二次修正）。
		markFailed := func() {
			symbolFailed = true
			if ctx.Err() != nil {
				deadlineHit = true
			}
		}
		// ctxDead 是**階段之間**的守衛：ctx 到期後這檔剩下的階段一定失敗，不必再送。
		// 這是「逾時後不再送出注定失敗的請求」這條原則縮到單檔內——
		// 先前只做到標的粒度，deadline 落在 dividends 時仍會對同一檔再送一次 reductions。
		// 那次呼叫即使立刻失敗也有代價：rateLimiter.wait 是**先推進 next 才判 ctx**
		// （finmind.go:68-88），所以它會燒掉一個 12 秒的節流槽，而那個 limiter 全 repo 共用。
		//
		// **skip 必須連著 markFailed**：只跳過不記的話，dividends 成功後才到期的那檔會變成
		// 「處理完且沒失敗」，但它的減資沒查、事件沒寫——拿一次請求換一個新的漏報。
		//
		// **呼叫順序：先判斷「還有工作要做」，再呼叫 ctxDead。**
		// 寫成 `!ctxDead() && len(actions) > 0` 的話，在「dividends 一般失敗 →
		// reductions 成功回空集合 → ctx 才到期」時，明明沒有東西可跳過卻會採樣到已到期的
		// ctx 並設 deadlineHit，把一般失敗誤標成逾時的問題就復活了。
		ctxDead := func() bool {
			if ctx.Err() == nil {
				return false
			}
			markFailed()
			return true
		}

		if a.dividends != nil {
			got, err := a.dividends.FetchDividends(ctx, symbol)
			if err != nil {
				markFailed()
				a.log.Warn("fetch dividends failed", zap.String("symbol", symbol), zap.Error(err))
			} else {
				actions = append(actions, got...)
			}
		}
		if a.reductions != nil && !ctxDead() {
			got, err := a.reductions.FetchCapitalReductions(ctx, symbol)
			if err != nil {
				markFailed()
				a.log.Warn("fetch capital reductions failed", zap.String("symbol", symbol), zap.Error(err))
			} else {
				actions = append(actions, got...)
			}
		}

		if len(actions) > 0 && !ctxDead() {
			if err := a.actions.Upsert(ctx, actions); err != nil {
				// 同一檔前面已經計過失敗就不重複計（見函式註解的 failed 說明）。
				markFailed()
				a.log.Warn("upsert per-symbol events failed", zap.String("symbol", symbol), zap.Error(err))
			} else {
				events += len(actions)
				// Upsert 成功之後**必定**還有重算要做，所以這裡直接問 ctxDead 就好，
				// 不需要像上面兩處那樣先判斷「還有沒有工作」。
				if ctxDead() {
					// **跳過重算，但不能跳過訊號。** 事件已經寫進去、係數沒跟上，
					// 這檔的價格現在是不一致的——job 層的 partial ＋ ctx 錯誤
					// 只說得出「這輪沒跑完」，說不出**是哪一檔**不一致，那要靠這行 log。
					// 這也比原本那行「recompute failed: context deadline exceeded」準確：
					// 不是重算失敗，是預算用完、沒來得及重算。下一輪重跑會自癒（Upsert 冪等）。
					a.log.Error("逾時，事件已寫入但未重算，該檔還原係數暫時落後",
						zap.String("symbol", symbol))
				} else if err := a.RecomputeSymbol(ctx, symbol); err != nil {
					// **重算失敗要算這檔失敗**：事件已經寫進去了，但 K 棒的還原係數沒跟上，
					// 這檔的價格從此不一致——比「抓不到事件」更嚴重，不能只留一行 log
					// 等人工翻。RecomputeAffected 走的也是這條原則（會把 firstErr 往上傳）。
					// symbolFailed 之前可能已經是 true，重複設定不會讓 failed 多加一次。
					markFailed()
					a.log.Error("recompute after per-symbol events failed",
						zap.String("symbol", symbol), zap.Error(err))
				}
			}
		}

		processed++
		if symbolFailed {
			failed++
		}
	}
	// 迴圈只在**每輪開頭**檢查 ctx，所以 deadline 落在最後一檔的請求裡時不會再進下一輪，
	// ctxErr 會留在 nil：那一檔的失敗有計進 failed，但「為什麼失敗」不會往上傳，
	// job_runs 就會出現「partial 但 error 欄空白」，與 api-reference.md 的契約不符。
	// 收尾補一次檢查。
	//
	// **守衛看的是 deadlineHit 而不是整輪的 failed**：`failed > 0` 只說明「這輪有檔失敗」，
	// 不說明失敗的原因。先前有檔因為一般 API 錯誤失敗、最後一檔跑完之後預算才到期時，
	// 用 failed 推斷會把那輪誤標成逾時，job_runs.error 寫著 context deadline exceeded，
	// 讀的人會去調 timeout_sec / shard_count，但真正該看的是資料源錯誤
	// （2026-08-24 review）。
	// deadlineHit 只在「某檔失敗時 ctx 已經到期」才為真，正好涵蓋
	// 「deadline 落在最後一檔請求裡」的情境，又不會誤收一般失敗。
	// ctx.Err() 一旦非 nil 就不會再變回 nil，所以這裡取值安全。
	if ctxErr == nil && deadlineHit {
		ctxErr = ctx.Err()
	}
	if failed > 0 {
		a.log.Warn("逐檔事件同步有標的失敗",
			zap.Int("failed", failed), zap.Int("processed", processed), zap.Int("planned", len(symbols)))
	}
	a.log.Info("逐檔事件同步完成",
		zap.Int("events", events), zap.Int("processed", processed), zap.Int("planned", len(symbols)))
	return processed, failed, ctxErr
}
